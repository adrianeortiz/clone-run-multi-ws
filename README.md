# Cross-Workspace Test Run Migration

This tool clones ALL test runs from a Qase project in Workspace A (Token A) to a project in Workspace B (Token B) that were created after a specified date, handling different case IDs via either a target-side custom field or a CSV mapping.

## Features

- **Dual HTTP clients** for source and target workspaces
- **Date filtering** to migrate runs created after a specific date
- **Automatic run creation** in target project
- **Two mapping modes**: custom field or CSV file
- **Bulk posting** with chunking and retries
- **Dry-run mode** for testing
- **Status translation** support
- **Environment-driven configuration**
- **Clear logging** (no secrets in logs)

## Environment Variables

### Required

- `QASE_SOURCE_API_TOKEN` - API token for source workspace
- `QASE_SOURCE_PROJECT` - Source project code
- `QASE_TARGET_API_TOKEN` - API token for target workspace
- `QASE_TARGET_PROJECT` - Target project code

### Optional

- `QASE_SOURCE_API_BASE` - Source API base URL (default: https://api.qase.io)
- `QASE_TARGET_API_BASE` - Target API base URL (default: https://api.qase.io)
- `QASE_AFTER_DATE` - Only migrate runs created after this date (RFC3339 format, default: 2025-08-18T00:00:00Z)
- `QASE_MATCH_MODE` - Mapping mode: `custom_field` or `csv` (default: custom_field)
- `QASE_CF_ID` - Custom field ID for custom_field mode (required if using custom_field)
- `QASE_MAPPING_CSV` - Path to CSV mapping file (required if using csv mode)
- `QASE_DRY_RUN` - Dry run mode: `true` or `false` (default: true)
- `QASE_BULK_SIZE` - Bulk posting chunk size (default: 200)
- `QASE_STATUS_MAP` - Status translation mapping (e.g., "passed:passed,failed:failed")

## Usage

### Custom Field Mapping Mode

```bash
export QASE_SOURCE_API_TOKEN="your_source_token"
export QASE_SOURCE_PROJECT="your_source_project"
export QASE_TARGET_API_TOKEN="your_target_token"
export QASE_TARGET_PROJECT="your_target_project"
export QASE_CF_ID="123"
export QASE_AFTER_DATE="2025-08-18T00:00:00Z"
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
export QASE_AFTER_DATE="2025-08-18T00:00:00Z"
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

The included workflow (`.github/workflows/cross-workspace.yml`) provides a manual trigger with inputs for:
- Date filtering (runs after specified date)
- Mapping mode selection
- Custom field ID
- Dry run toggle

Required secrets:
- `QASE_TOKEN_WS_A` - Source workspace token
- `QASE_TOKEN_WS_B` - Target workspace token

Required variables:
- `QASE_SOURCE_PROJECT` - Source project code
- `QASE_TARGET_PROJECT` - Target project code

## Architecture

The code is organized into packages:

- `api/` - HTTP client wrapper for Qase API
- `qase/` - Qase-specific data structures and API calls
- `mapping/` - Case ID mapping logic
- `main.go` - Main orchestration and configuration

## Error Handling

- **Retries**: HTTP 429 and 5xx errors are retried with exponential backoff
- **Validation**: Environment variables are validated on startup
- **Logging**: Clear error messages without exposing secrets
- **Graceful degradation**: Invalid mappings are skipped with warnings
