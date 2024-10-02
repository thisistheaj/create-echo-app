#!/bin/bash

# Get the current directory name as the project name
PROJECT_NAME=$(basename "$PWD")

# Check if we're in a Git repository and remove it if found
if [ -d .git ]; then
    # Remove the original Git history
    rm -rf .git
fi

# Initialize a new repository
git init

# Replace the template project name with the new project name in all files
find . -type f -not -path '*/\.*' -type f -print0 | xargs -0 sed -i '' "s/your_project_name/$PROJECT_NAME/g"

# Initialize the Go module with the new project name
go mod init "$PROJECT_NAME"

# Generate templ files if needed
if command -v templ &> /dev/null; then
    templ generate
else
    echo "Warning: 'templ' command not found. Skipping template generation."
    echo "You can install templ by running:"
    echo ""
    echo "  brew install templ"
    echo ""
fi

# must be run AFTER `templ generate`, otherwise templ dependencies will not be found
go mod tidy

# Remove the setup script itself
rm -- "$0"

echo "Project '$PROJECT_NAME' has been set up successfully!"
echo "Next steps:"
echo "1. Review and update the README.md file"
echo "2. Commit the changes: git add . && git commit -m 'Initial commit'"
echo "3. Create a new repository on GitHub (without README, license, or .gitignore)"
echo "4. Push to the new repository: git remote add origin <new-repo-url> && git push -u origin main"