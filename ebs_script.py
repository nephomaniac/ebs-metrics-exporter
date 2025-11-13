#!/usr/bin/env python3

# Copyright Amazon.com, Inc. and its affiliates. All Rights Reserved.
#
# Licensed under the MIT License. See the LICENSE accompanying this file
# for the specific language governing permissions and limitations under
# the License.

"""
EBS Performance Metrics Exporter for Node Exporter

This script exports EBS volume and EC2 instance performance metrics in
Prometheus textfile collector format for use with node-exporter.

Usage:
    ebs_performance_exporter.py --device /dev/nvme1n1 --output /var/lib/node_exporter/textfile_collector/ebs_performance.prom
    ebs_performance_exporter.py --device /dev/nvme1n1 --interval 60 --output /var/lib/node_exporter/textfile_collector/ebs_performance.prom
"""

from __future__ import print_function
import argparse
import json
import os
import re
import sys
import time
import signal
from ctypes import Structure, Array, c_uint8, c_uint16, c_uint32, c_uint64, \
    c_char, addressof, sizeof
from fcntl import ioctl

# NVMe IOCTL constants
NVME_ADMIN_IDENTIFY = 0x06
NVME_GET_LOG_PAGE = 0x02
NVME_IOCTL_ADMIN_CMD = 0xC0484E41

# Amazon EBS NVMe constants
AMZN_NVME_EBS_MN = "Amazon Elastic Block Store"
AMZN_NVME_STATS_LOGPAGE_ID = 0xD0
AMZN_NVME_STATS_MAGIC = 0x3C23B510
AMZN_NVME_VID = 0x1D0F


class structure_dict_mixin:
    def to_dict(self):
        return {
            field[0]: getattr(self, field[0])
            for field in self._fields_
            if not field[0].startswith("_") and
            not isinstance(getattr(self, field[0]), (Structure, Array))
        }


class nvme_admin_command(Structure):
    _pack_ = 1
    _fields_ = [("opcode", c_uint8),
                ("flags", c_uint8),
                ("cid", c_uint16),
                ("nsid", c_uint32),
                ("_reserved0", c_uint64),
                ("mptr", c_uint64),
                ("addr", c_uint64),
                ("mlen", c_uint32),
                ("alen", c_uint32),
                ("cdw10", c_uint32),
                ("cdw11", c_uint32),
                ("cdw12", c_uint32),
                ("cdw13", c_uint32),
                ("cdw14", c_uint32),
                ("cdw15", c_uint32),
                ("_reserved1", c_uint64)]


class nvme_identify_controller_amzn_vs(Structure):
    _pack_ = 1
    _fields_ = [("bdev", c_char * 32),
                ("_reserved0", c_char * (1024 - 32))]


class nvme_identify_controller(Structure):
    _pack_ = 1
    _fields_ = [("vid", c_uint16),
                ("ssvid", c_uint16),
                ("sn", c_char * 20),
                ("mn", c_char * 40),
                ("fr", c_char * 8),
                ("rab", c_uint8),
                ("ieee", c_uint8 * 3),
                ("mic", c_uint8),
                ("mdts", c_uint8),
                ("_reserved0", c_uint8 * (256 - 78)),
                ("oacs", c_uint16),
                ("acl", c_uint8),
                ("aerl", c_uint8),
                ("frmw", c_uint8),
                ("lpa", c_uint8),
                ("elpe", c_uint8),
                ("npss", c_uint8),
                ("avscc", c_uint8),
                ("_reserved1", c_uint8 * (512 - 265)),
                ("sqes", c_uint8),
                ("cqes", c_uint8),
                ("_reserved2", c_uint16),
                ("nn", c_uint32),
                ("oncs", c_uint16),
                ("fuses", c_uint16),
                ("fna", c_uint8),
                ("vwc", c_uint8),
                ("awun", c_uint16),
                ("awupf", c_uint16),
                ("nvscc", c_uint8),
                ("_reserved3", c_uint8 * (704 - 531)),
                ("_reserved4", c_uint8 * (2048 - 704)),
                ("psd", c_uint8 * (32 * 32)),
                ("vs", nvme_identify_controller_amzn_vs)]


class nvme_histogram_bin(Structure, structure_dict_mixin):
    _pack_ = 1
    _fields_ = [("lower", c_uint64),
                ("upper", c_uint64),
                ("count", c_uint32),
                ("_reserved0", c_uint32)]


class ebs_nvme_histogram(Structure, structure_dict_mixin):
    _pack_ = 1
    _fields_ = [("num_bins", c_uint64),
                ("bins", nvme_histogram_bin * 64)]


class nvme_get_amzn_stats_logpage(Structure, structure_dict_mixin):
    _pack_ = 1
    _fields_ = [("_magic", c_uint32),
                ("_reserved0", c_char * 4),
                ("total_read_ops", c_uint64),
                ("total_write_ops", c_uint64),
                ("total_read_bytes", c_uint64),
                ("total_write_bytes", c_uint64),
                ("total_read_time", c_uint64),
                ("total_write_time", c_uint64),
                ("ebs_volume_performance_exceeded_iops", c_uint64),
                ("ebs_volume_performance_exceeded_tp", c_uint64),
                ("ebs_instance_performance_exceeded_iops", c_uint64),
                ("ebs_instance_performance_exceeded_tp", c_uint64),
                ("volume_queue_length", c_uint64),
                ("_reserved1", c_char * 416),
                ("read_io_latency_histogram", ebs_nvme_histogram),
                ("write_io_latency_histogram", ebs_nvme_histogram),
                ("_reserved2", c_char * 496)]


class EBSPerformanceExporter:
    def __init__(self, device, output_file=None):
        self.device = device
        self.output_file = output_file
        self.volume_id = None
        self.prev_stats = None
        self.running = True

    def _nvme_ioctl(self, admin_cmd):
        """Execute NVMe IOCTL command"""
        with open(self.device, "r") as dev:
            try:
                ioctl(dev, NVME_IOCTL_ADMIN_CMD, admin_cmd)
            except (OSError) as err:
                print(f"Failed to issue nvme cmd, err: {err}", file=sys.stderr)
                sys.exit(1)

    def _get_volume_id(self):
        """Get the EBS volume ID from the device"""
        id_ctrl = nvme_identify_controller()
        admin_cmd = nvme_admin_command(
            opcode=NVME_ADMIN_IDENTIFY,
            addr=addressof(id_ctrl),
            alen=sizeof(id_ctrl),
            cdw10=1
        )
        self._nvme_ioctl(admin_cmd)

        if id_ctrl.vid != AMZN_NVME_VID or id_ctrl.mn.decode().strip() != AMZN_NVME_EBS_MN:
            raise TypeError(f"[ERROR] Not an EBS device: {self.device}")

        vol = id_ctrl.sn.decode()
        if vol.startswith("vol") and vol[3] != "-":
            vol = "vol-" + vol[3:]
        return vol.strip()

    def _query_stats(self):
        """Query statistics from the EBS NVMe device"""
        stats = nvme_get_amzn_stats_logpage()
        admin_cmd = nvme_admin_command(
            opcode=NVME_GET_LOG_PAGE,
            addr=addressof(stats),
            alen=sizeof(stats),
            nsid=1,
            cdw10=(AMZN_NVME_STATS_LOGPAGE_ID | (1024 << 16))
        )
        self._nvme_ioctl(admin_cmd)

        if stats._magic != AMZN_NVME_STATS_MAGIC:
            raise TypeError(f"[ERROR] Not an EBS device: {self.device}")

        return stats

    def _get_state_file_path(self):
        """Get the path to the state file in /tmp"""
        # Create a unique filename based on the device name
        device_name = self.device.replace('/dev/', '').replace('/', '_')
        return f"/tmp/ebs_stats_{device_name}.json"

    def _read_state_file(self):
        """Read previous metric values from the state file in /tmp"""
        state_file = self._get_state_file_path()

        if not os.path.exists(state_file):
            return None, None, None

        try:
            with open(state_file, 'r') as f:
                data = json.load(f)

            prev_iops = data.get('ebs_volume_performance_exceeded_iops_total')
            prev_tp = data.get('ebs_volume_performance_exceeded_throughput_total')
            prev_timestamp = data.get('timestamp')

            return prev_iops, prev_tp, prev_timestamp
        except Exception as e:
            print(f"Warning: Could not read state file: {e}", file=sys.stderr)
            return None, None, None

    def _write_state_file(self, iops_total, tp_total, timestamp=None):
        """Write current metric values to the state file in /tmp"""
        state_file = self._get_state_file_path()

        try:
            data = {
                'ebs_volume_performance_exceeded_iops_total': iops_total,
                'ebs_volume_performance_exceeded_throughput_total': tp_total,
                'timestamp': timestamp if timestamp is not None else time.time()
            }

            # Write atomically using a temp file
            temp_file = f"{state_file}.tmp"
            with open(temp_file, 'w') as f:
                json.dump(data, f, indent=2)
            os.rename(temp_file, state_file)
        except Exception as e:
            print(f"Warning: Could not write state file: {e}", file=sys.stderr)
            if os.path.exists(temp_file):
                os.remove(temp_file)

    def _read_previous_metrics(self):
        """Read previous metric values from the output file"""
        if not self.output_file or not os.path.exists(self.output_file):
            return None, None

        try:
            with open(self.output_file, 'r') as f:
                content = f.read()

            # Parse the metrics using regex
            # Looking for lines like: ebs_volume_performance_exceeded_iops_total{device="nvme1n1",volume_id="vol-xxx"} 12345
            iops_match = re.search(r'ebs_volume_performance_exceeded_iops_total\{[^}]+\}\s+(\d+)', content)
            tp_match = re.search(r'ebs_volume_performance_exceeded_throughput_total\{[^}]+\}\s+(\d+)', content)

            prev_iops = int(iops_match.group(1)) if iops_match else None
            prev_tp = int(tp_match.group(1)) if tp_match else None

            return prev_iops, prev_tp
        except Exception as e:
            print(f"Warning: Could not read previous metrics: {e}", file=sys.stderr)
            return None, None

    def _calculate_diff(self, curr_stats):
        """Calculate the difference between current and previous stats"""
        if self.prev_stats is None:
            return curr_stats

        diff = nvme_get_amzn_stats_logpage()
        diff.volume_queue_length = curr_stats.volume_queue_length

        # Calculate differences for counters
        diff.total_read_ops = curr_stats.total_read_ops - self.prev_stats.total_read_ops
        diff.total_write_ops = curr_stats.total_write_ops - self.prev_stats.total_write_ops
        diff.total_read_bytes = curr_stats.total_read_bytes - self.prev_stats.total_read_bytes
        diff.total_write_bytes = curr_stats.total_write_bytes - self.prev_stats.total_write_bytes
        diff.total_read_time = curr_stats.total_read_time - self.prev_stats.total_read_time
        diff.total_write_time = curr_stats.total_write_time - self.prev_stats.total_write_time
        diff.ebs_volume_performance_exceeded_iops = curr_stats.ebs_volume_performance_exceeded_iops - self.prev_stats.ebs_volume_performance_exceeded_iops
        diff.ebs_volume_performance_exceeded_tp = curr_stats.ebs_volume_performance_exceeded_tp - self.prev_stats.ebs_volume_performance_exceeded_tp
        diff.ebs_instance_performance_exceeded_iops = curr_stats.ebs_instance_performance_exceeded_iops - self.prev_stats.ebs_instance_performance_exceeded_iops
        diff.ebs_instance_performance_exceeded_tp = curr_stats.ebs_instance_performance_exceeded_tp - self.prev_stats.ebs_instance_performance_exceeded_tp

        return diff

    def _format_prometheus_metrics(self, stats, interval=None, prev_iops_total=None, prev_tp_total=None):
        """Format statistics as Prometheus metrics"""
        lines = []

        # Get device name without /dev/ prefix
        device_name = self.device.replace('/dev/', '')

        # Helper function to add metrics
        def add_metric(metric_name, value, help_text, metric_type="counter", labels=None):
            if labels is None:
                labels = {}
            labels['device'] = device_name
            labels['volume_id'] = self.volume_id

            label_str = ','.join([f'{k}="{v}"' for k, v in labels.items()])

            lines.append(f"# HELP {metric_name} {help_text}")
            lines.append(f"# TYPE {metric_name} {metric_type}")
            lines.append(f"{metric_name}{{{label_str}}} {value}")
            lines.append("")

        # Volume performance exceeded metrics (microseconds)
        add_metric(
            "ebs_volume_performance_exceeded_iops_total",
            stats.ebs_volume_performance_exceeded_iops,
            "Total time in microseconds that the EBS volume IOPS limit was exceeded",
            "counter"
        )

        add_metric(
            "ebs_volume_performance_exceeded_throughput_total",
            stats.ebs_volume_performance_exceeded_tp,
            "Total time in microseconds that the EBS volume throughput limit was exceeded",
            "counter"
        )

        # Determine check values based on whether counters increased
        # If previous values exist and current > previous, return 1, otherwise 0
        iops_exceeded_check = 0
        if prev_iops_total is not None:
            if stats.ebs_volume_performance_exceeded_iops > prev_iops_total:
                iops_exceeded_check = 1

        tp_exceeded_check = 0
        if prev_tp_total is not None:
            if stats.ebs_volume_performance_exceeded_tp > prev_tp_total:
                tp_exceeded_check = 1

        add_metric(
            "ebs_volume_throughput_exceeded_check",
            tp_exceeded_check,
            "Reports whether an application consistently attempted to drive throughput that exceeds the volume's provisioned throughput performance within the last collection interval",
            "gauge"
        )

        add_metric(
            "ebs_volume_iops_exceeded_check",
            iops_exceeded_check,
            "Reports whether an application consistently attempted to drive IOPS that exceeds the volume's provisioned IOPS performance within the last collection interval",
            "gauge"
        )

        # Instance performance exceeded metrics (microseconds)
        add_metric(
            "ebs_instance_performance_exceeded_iops_total",
            stats.ebs_instance_performance_exceeded_iops,
            "Total time in microseconds that the EC2 instance EBS IOPS limit was exceeded",
            "counter"
        )

        add_metric(
            "ebs_instance_performance_exceeded_throughput_total",
            stats.ebs_instance_performance_exceeded_tp,
            "Total time in microseconds that the EC2 instance EBS throughput limit was exceeded",
            "counter"
        )
        

        # If we have interval data, calculate rates
        if interval and interval > 0:
            # Calculate percentage of time exceeded during the interval
            interval_us = interval * 1000000  # Convert to microseconds

            volume_iops_exceeded_pct = (stats.ebs_volume_performance_exceeded_iops / interval_us) * 100 if interval_us > 0 else 0
            volume_tp_exceeded_pct = (stats.ebs_volume_performance_exceeded_tp / interval_us) * 100 if interval_us > 0 else 0
            instance_iops_exceeded_pct = (stats.ebs_instance_performance_exceeded_iops / interval_us) * 100 if interval_us > 0 else 0
            instance_tp_exceeded_pct = (stats.ebs_instance_performance_exceeded_tp / interval_us) * 100 if interval_us > 0 else 0

            add_metric(
                "ebs_volume_performance_exceeded_iops_percent",
                round(volume_iops_exceeded_pct, 2),
                "Percentage of time that the EBS volume IOPS limit was exceeded during the last interval",
                "gauge"
            )

            add_metric(
                "ebs_volume_performance_exceeded_throughput_percent",
                round(volume_tp_exceeded_pct, 2),
                "Percentage of time that the EBS volume throughput limit was exceeded during the last interval",
                "gauge"
            )
            

            add_metric(
                "ebs_instance_performance_exceeded_iops_percent",
                round(instance_iops_exceeded_pct, 2),
                "Percentage of time that the EC2 instance EBS IOPS limit was exceeded during the last interval",
                "gauge"
            )

            add_metric(
                "ebs_instance_performance_exceeded_throughput_percent",
                round(instance_tp_exceeded_pct, 2),
                "Percentage of time that the EC2 instance EBS throughput limit was exceeded during the last interval",
                "gauge"
            )

        # Additional useful metrics
        add_metric(
            "ebs_total_read_ops_total",
            stats.total_read_ops,
            "Total number of read operations",
            "counter"
        )

        add_metric(
            "ebs_total_write_ops_total",
            stats.total_write_ops,
            "Total number of write operations",
            "counter"
        )

        add_metric(
            "ebs_total_read_bytes_total",
            stats.total_read_bytes,
            "Total bytes read",
            "counter"
        )

        add_metric(
            "ebs_total_write_bytes_total",
            stats.total_write_bytes,
            "Total bytes written",
            "counter"
        )

        add_metric(
            "ebs_volume_queue_length",
            stats.volume_queue_length,
            "Current volume queue length",
            "gauge"
        )

        return '\n'.join(lines)

    def _write_metrics(self, content):
        """Write metrics to output file or stdout"""
        if self.output_file:
            # Write to temporary file and then rename atomically
            temp_file = f"{self.output_file}.tmp"
            try:
                with open(temp_file, 'w') as f:
                    f.write(content)
                os.rename(temp_file, self.output_file)
            except Exception as e:
                print(f"Error writing to {self.output_file}: {e}", file=sys.stderr)
                if os.path.exists(temp_file):
                    os.remove(temp_file)
        else:
            print(content)

    def _signal_handler(self, sig, frame):
        """Handle SIGINT/SIGTERM gracefully"""
        self.running = False
        sys.exit(0)

    def export_once(self):
        """Export metrics once and exit"""
        try:
            self.volume_id = self._get_volume_id()
            current_timestamp = time.time()

            # Try to read previous metrics from state file first
            prev_iops_total, prev_tp_total, prev_timestamp = self._read_state_file()

            # If state file doesn't exist or is invalid, fall back to reading from output file
            if prev_iops_total is None or prev_tp_total is None:
                prev_iops_total, prev_tp_total = self._read_previous_metrics()

            # Calculate interval from elapsed time if we have a previous timestamp
            interval = None
            if prev_timestamp is not None:
                interval = current_timestamp - prev_timestamp

            stats = self._query_stats()
            content = self._format_prometheus_metrics(stats, interval=interval, prev_iops_total=prev_iops_total, prev_tp_total=prev_tp_total)
            self._write_metrics(content)

            # Save current values to state file for next run
            self._write_state_file(stats.ebs_volume_performance_exceeded_iops, stats.ebs_volume_performance_exceeded_tp, current_timestamp)
        except Exception as e:
            print(f"Error exporting metrics: {e}", file=sys.stderr)
            sys.exit(1)

    def export_continuous(self, interval):
        """Export metrics continuously at the specified interval"""
        signal.signal(signal.SIGINT, self._signal_handler)
        signal.signal(signal.SIGTERM, self._signal_handler)

        try:
            self.volume_id = self._get_volume_id()
            print(f"Exporting metrics for volume {self.volume_id} every {interval} seconds", file=sys.stderr)
            print(f"Output: {self.output_file if self.output_file else 'stdout'}", file=sys.stderr)
            print("Press Ctrl+C to stop", file=sys.stderr)

            while self.running:
                curr_stats = self._query_stats()
                diff_stats = self._calculate_diff(curr_stats)
                self.prev_stats = curr_stats

                content = self._format_prometheus_metrics(diff_stats, interval)
                self._write_metrics(content)

                time.sleep(interval)

        except Exception as e:
            print(f"Error exporting metrics: {e}", file=sys.stderr)
            sys.exit(1)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Export EBS performance metrics in Prometheus textfile collector format"
    )
    parser.add_argument(
        "--device",
        required=True,
        help="NVMe device to monitor (e.g., /dev/nvme1n1)"
    )
    parser.add_argument(
        "--output",
        help="Output file path (default: stdout). Recommended: /var/lib/node_exporter/textfile_collector/ebs_performance.prom"
    )
    parser.add_argument(
        "--interval",
        type=int,
        default=0,
        help="Collection interval in seconds (0 = run once and exit, >0 = continuous collection)"
    )

    args = parser.parse_args()

    exporter = EBSPerformanceExporter(args.device, args.output)

    if args.interval > 0:
        exporter.export_continuous(args.interval)
    else:
        exporter.export_once()