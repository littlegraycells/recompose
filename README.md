# Recompose

Recompose is a tiny CLI utility designed to automate the extraction of hardcoded environment variables from Docker Compose files into a structured `.env` file. It modifies the `services` block in compose files to use variable interpolation (`${VAR_NAME}`) and organizes the output into a clean, hierarchical directory.

## Features

1. **Tree-Aware Refactoring:** Parses YAML into an in-memory AST (Abstract Syntax Tree) to preserve original comments, indentation, and formatting.
1. **Smart .env Organization:**
    1. **Global Section:** Identifies variables shared across multiple services to reduce redundancy.
    1. **Service Sections:** Groups unique variables under specific service headers.
1. **Dry Run Support:** View all proposed changes in a formatted Unicode table before any files are written to disk.
1. **Safety First:** Always generates files in a dedicated `recompose/` directory relative to your source file, ensuring your original configuration remains untouched.
1. **Interactive Confirmation:** Prompts for final approval before writing, allowing for a safe review of the transformation table.

## Installation

Download the `recompose` binary for your platform from the Releases page and ensure it has execution permissions:

```bash
chmod +x recompose

# Optional: move to your bin folder
mv recompose /usr/local/bin/
```

## Usage

### Basic Command

```
./recompose -F docker-compose.yml
```

### Options

| Flag                | Shorthand | Default  | Description                                            |
| ------------------- | --------- | -------- | ------------------------------------------------------ |
| \--file             | \-F       | Required | Path to the source docker-compose.yml file.            |
| \--generate-compose |           | TRUE     | Creates a refactored compose.yml in the output folder. |
| \--generate-env     |           | TRUE     | Creates a formatted .env file in the output folder.    |
| \--version          | \-V       |          | Prints the current version of the binary.              |
| \--help             | \-h       |          | Displays all available commands and flags.             |


### Example: Dry Run
To see the proposed changes without creating any files, set both generation flags to false:

```
./recompose -F docker-compose.yml -C=false -E=false
```

## Output Structure
When executed, `recompose` creates a directory named `recompose` in the same location as your input file:

```
.
├── docker-compose.yml     <-- Your original file (untouched)
└── recompose/
    ├── .env               <-- New organized environment file
    └── compose.yml        <-- New refactored compose file
```

## Generated .env file example
```
# GLOBAL VARIABLES
DB_PASSWORD=secret_pass
LOG_LEVEL=info

# SERVICE: WEB
PORT=${PORT}
VIRTUAL_HOST=myapp.local

# SERVICE: WORKER
CONCURRENCY=5
```

## Validations
The tool performs several rigorous checks before processing:
  1. **Path Resolution**: Resolves the absolute path of the input file to ensure the recompose/ folder is placed correctly.
  1. **Schema Integrity**: Validates that the file is a valid YAML document.
  1. **Compose Compliance**: Confirms the services block exists and contains at least one active service definition.
  1. **Targeted Scanning**: Only modifies values within environment blocks, ignoring other configuration keys.

## Contributing
Contributions are welcome. If you wish to suggest improvements or report bugs:
   1. Fork the repository.
   1.  Create your feature branch (git checkout -b feature/AmazingFeature).
   1.  Commit your changes.
   1.  Push to the branch.
   1.  Open a Pull Request.

## License
Distributed under the MIT License.
