# Bencher Diagnostics

Load this reference when the dashboard says `No thresholds found`, when a task involves thresholds, or when verifying that the latest report has zero `threshold: null` measures. Check the report JSON first: a successful `bencher run` can still have one unthresholded measure.

List unthresholded measures in the latest report:

```bash
project=ddfe58db-e86d-49b8-a6c7-60fc46eabf0b
branch=pr-<number>
testbed=bencher-intel-v1-go-1-27
api=https://api.bencher.dev/v0
curl -fsS "$api/projects/$project/reports?branch=$branch&testbed=$testbed&sort=date_time&direction=desc&per_page=1" |
	jq -r '.[0].results[][] as $bench |
		$bench.measures[] |
		select(.threshold == null) |
		[$bench.benchmark.name, .measure.slug, .measure.name, .metric.value] |
		@tsv'
```

Summarize threshold coverage by measure:

```bash
curl -fsS "$api/projects/$project/reports?branch=$branch&testbed=$testbed&sort=date_time&direction=desc&per_page=1" |
	jq -r '[.[0].results[][] as $bench |
		$bench.measures[] |
		{measure:.measure.slug, has_threshold:(.threshold != null)}] |
		group_by(.measure)[] |
		[.[0].measure, length, (map(select(.has_threshold)) | length), (map(select(.has_threshold | not)) | length)] |
		@tsv'
```

List the active threshold models:

```bash
curl -fsS "$api/projects/$project/thresholds?branch=$branch&testbed=$testbed" |
	jq -r '.[] |
		[.measure.slug, .measure.name, .model.test, .model.upper_boundary, .model.min_sample_size, .model.max_sample_size] |
		@tsv' |
	sort
```

Inspect Actions logs for the actual `bencher run` request and report URL:

```bash
log_file=$(mktemp)
gh run view <run-id> --repo invowk/invowk --log > "$log_file"
rg -n 'Bencher New Report|thresholds|View report|No thresholds|Job status|BENCHER_|--threshold|--start-point' "$log_file"
```
