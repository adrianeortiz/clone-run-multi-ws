# Cross-Workspace Test Run Migration

This tool migrates test execution results from a Qase project in Workspace A (Token A) to a project in Workspace B (Token B) that were executed after a specified date, handling different case IDs via either a target-side custom field or a CSV mapping.

**Note**: This tool uses the Qase Results API (`/v1/result/{code}`) to fetch test execution data directly, making it more efficient than traditional run-based migration approaches.

## Features

- **Results-based migration** using Qase Results API for efficient data fetching
- **Date filtering** to migrate test results executed after a specific date
- **Automatic run creation** in target project with meaningful titles
- **Two mapping modes**: custom field or CSV file
- **Bulk posting** with chunking and retries
- **Dry-run mode** for testing
- **Status translation** support
- **Idempotent operation** - safe to re-run without creating duplicates
- **Environment-driven configuration**
- **Clear logging** (no secrets in logs)
- **Concurrent processing** for faster migration

## Environment Variables

### Required

- `QASE_SOURCE_API_TOKEN` - API token for source workspace
- `QASE_SOURCE_PROJECT` - Source project code
- `QASE_TARGET_API_TOKEN` - API token for target workspace
- `QASE_TARGET_PROJECT` - Target project code

### Optional

- `QASE_SOURCE_API_BASE` - Source API base URL (default: https://api.qase.io)
- `QASE_TARGET_API_BASE` - Target API base URL (default: https://api.qase.io)
- `QASE_AFTER_DATE` - Only migrate test results executed after this date (Unix timestamp, default: 1755500400)
- `QASE_MATCH_MODE` - Mapping mode: `custom_field` or `csv` (default: custom_field)
- `QASE_CF_ID` - Custom field ID for custom_field mode (required if using custom_field)
- `QASE_MAPPING_CSV` - Path to CSV mapping file (required if using csv mode)
- `QASE_DRY_RUN` - Dry run mode: `true` or `false` (default: true)
- `QASE_BULK_SIZE` - Bulk posting chunk size (default: 200)
- `QASE_STATUS_MAP` - Status translation mapping (e.g., "passed:passed,failed:failed")
- `QASE_IDEMPOTENT` - Idempotent mode: `true` or `false` (default: true)

## Usage

### Custom Field Mapping Mode

```bash
export QASE_SOURCE_API_TOKEN="your_source_token"
export QASE_SOURCE_PROJECT="your_source_project"
export QASE_TARGET_API_TOKEN="your_target_token"
export QASE_TARGET_PROJECT="your_target_project"
export QASE_CF_ID="123"
export QASE_AFTER_DATE="1755500400"
export QASE_DRY_RUN="true"

go run .
```

### CSV Mapping Mode

```bash
export QASE_SOURCE_API_TOKEN="your_source_token"
export QASE_SOURCE_PROJECT="your_source_project"
export QASE_TARGET_API_TOKEN="your_target_token"
export QASE_TARGET_PROJECT="your_target_project"
export QASE_MATCH_MODE="csv"
export QASE_MAPPING_CSV="./mapping.csv"
export QASE_AFTER_DATE="1755500400"
export QASE_DRY_RUN="true"

go run .
```

### CSV Mapping File Format

The CSV file should have the following format:

```csv
source_case_id,target_case_id
1,101
2,102
3,103
```

## Output

- **Console logs**: Progress information, run-by-run processing, and summary statistics
- **case_map.out.csv**: Generated mapping file showing source → target case ID mappings
- **Migration summary**: Total runs processed, successful/failed migrations, and result counts

## GitHub Actions

The migration pipeline (`.github/workflows/migration-pipeline.yml`) provides a comprehensive 3-step migration process:

1. **Analyze Project Data** - Analyzes source and target projects
2. **Fetch Test Results** - Uses Results API to fetch all test execution data
3. **Migrate Data** - Performs the actual migration with case mapping

**Manual trigger inputs:**
- Source/Target project codes
- Date filtering (Unix timestamp)
- Mapping mode selection (custom_field or csv)
- Custom field ID (for custom_field mode)
- Dry run toggle

**Required secrets:**
- `QASE_SOURCE_API_TOKEN` - Source workspace token
- `QASE_TARGET_API_TOKEN` - Target workspace token

**Required variables:**
- `QASE_SOURCE_PROJECT` - Source project code
- `QASE_TARGET_PROJECT` - Target project code

## Architecture

The code is organized into packages:

- `api/` - HTTP client wrapper for Qase API
- `qase/` - Qase-specific data structures and API calls (results, cases, runs)
- `mapping/` - Case ID mapping logic
- `utils/` - Utility functions for date parsing
- `tools/` - Helper scripts for custom field management
- `main.go` - Main orchestration and configuration

## How It Works

1. **Fetch Results**: Uses the Results API to get all test execution results after the specified date
2. **Group by Run**: Groups results by their original run ID to recreate run structure
3. **Map Cases**: Maps source case IDs to target case IDs using custom field or CSV mapping
4. **Create/Find Runs**: Creates new runs or finds existing ones in the target workspace (idempotent)
5. **Filter Results**: Filters out results that already exist in the target run (idempotent)
6. **Post Results**: Bulk posts only the new mapped results to the target runs

### Idempotent Behavior

When `QASE_IDEMPOTENT=true` (default):
- **Run Deduplication**: Checks if a run with the same title already exists before creating
- **Result Filtering**: Only posts results that don't already exist in the target run
- **Safe Re-runs**: You can safely re-run the migration without creating duplicates
- **Progress Tracking**: Shows how many results are new vs. already exist

When `QASE_IDEMPOTENT=false`:
- **Always Creates New Runs**: Creates new runs every time (legacy behavior)
- **Posts All Results**: Posts all results without checking for duplicates

## Error Handling

- **Retries**: HTTP 429 and 5xx errors are retried with exponential backoff
- **Validation**: Environment variables are validated on startup
- **Logging**: Clear error messages without exposing secrets
- **Graceful degradation**: Invalid mappings are skipped with warnings
