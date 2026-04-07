# Cost Analysis TODO

## Objective
Compare the cost of IOCTL-based EBS metrics collection vs CloudWatch-based collection for OpenShift's managed fleet.

## CloudWatch Operator Baseline
- Repository: https://github.com/aws/amazon-cloudwatch-agent-operator
- Uses AWS CloudWatch APIs to collect EBS metrics
- Costs include:
  - API calls to CloudWatch
  - CloudWatch metrics storage
  - Network egress charges (AWS → Dynatrace/Prometheus)
  - Data transfer within AWS

## IOCTL-Based Solution (This Project)
- Collects metrics directly from NVMe devices via IOCTL
- No AWS API calls required
- Costs include:
  - Minimal CPU/memory overhead per node
  - Network egress (metrics to Prometheus/Dynatrace)
  - Storage for time-series data

## Analysis Requirements

### Fleet Size Estimates Needed
- Total number of ROSA/OSD clusters
- Average number of nodes per cluster
- Average number of EBS volumes per node
- Metrics collection frequency (scrape interval)

### CloudWatch Cost Components
1. **API Requests**:
   - Queries per node per scrape interval
   - AWS CloudWatch API pricing: $0.01 per 1,000 requests
   - Calculate: (clusters × nodes × volumes × scrapes/day × 365) × $0.01/1000

2. **CloudWatch Metrics Storage**:
   - Custom metrics pricing: $0.30 per metric per month
   - Calculate: (clusters × nodes × volumes × metrics_per_volume) × $0.30 × 12

3. **Data Transfer Out**:
   - AWS → External (Dynatrace): $0.09/GB (first 10TB)
   - Estimate metric payload size and frequency

### IOCTL Solution Cost Components
1. **Compute Overhead**:
   - DaemonSet CPU/memory per node
   - Likely negligible compared to existing node-exporter

2. **Data Transfer Out**:
   - Same as CloudWatch (metrics still need to egress to Dynatrace)
   - Potentially same cost

3. **Operational Overhead**:
   - Maintenance of custom exporter
   - vs CloudWatch's managed service

### Cost Savings Hypothesis
- **Primary savings**: Eliminate AWS API calls and CloudWatch metrics storage
- **Secondary savings**: Potentially reduce network I/O if metrics are aggregated before egress
- **Trade-off**: Custom code maintenance vs AWS managed service

### Action Items
1. [ ] Get fleet size metrics from SRE Platform team
2. [ ] Calculate CloudWatch API call volume at current scrape intervals
3. [ ] Estimate CloudWatch metrics storage costs
4. [ ] Measure actual IOCTL solution resource overhead
5. [ ] Compare network egress costs (should be similar)
6. [ ] Create annual cost comparison spreadsheet
7. [ ] Factor in operational maintenance costs
8. [ ] Present findings to stakeholders

### Data Sources
- Fleet metrics: OCM, app-interface
- AWS pricing: https://aws.amazon.com/cloudwatch/pricing/
- Current monitoring costs: Finance team / Cost Explorer

### Success Criteria
Annual cost savings > (development cost + maintenance cost) over 2-3 year horizon
