#!/bin/bash

#
# ----- 1. Parse necessary variables and options -----
#

project_name=$(basename "$(pwd)")
environment_name="production"

VERBOSE=false
while getopts "v" opt; do
  case $opt in
    v)
      VERBOSE=true
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      exit 1
      ;;
  esac
done

#
# ----- 2. Run checks -----
#

# Check if Railway CLI is installed
if ! which railway &> /dev/null
then
    echo "Error: Railway CLI is not installed. Please install it first."
    echo "Visit https://docs.railway.app/develop/cli for installation instructions."
    exit 1
fi

# Read the railway token from a file named 'railway.token' in the home directory
if [ -f "$HOME/.railway/railway.token" ]; then
    railway_token=$(cat "$HOME/.railway/railway.token")
else
    echo "Error: railway.token file not found at '$HOME/.railway/railway.token'. Please create it with your Railway token by running:"
    echo ""
    echo -e '\techo <your_token_here> > $HOME/.railway/railway.token'
    echo ""
    exit 1
fi

#
# ----- 3. Define functions -----
#

function run_query {
    local query=$1
    local response=$(curl -s --request POST https://backboard.railway.app/graphql/v2 \
        -H "Authorization: Bearer $railway_token" \
        -H "Content-Type: application/json" \
        -d "{\"query\": $(echo "$query" | jq -R -s -c .)}")
    echo $response
}

function load_projects {
    local projects_query=$(cat <<EOF
query projects {
    projects {
        edges {
            node {
                id
                name
                environments {
                    edges {
                        node {
                            id
                            name
                        }
                    }
                }
                services {
                    edges {
                        node {
                            id
                            name
                        }
                    }
                }
            }
        }
    }
}
EOF
)
    local response=$(run_query "$projects_query")
    echo $response
}

function get_variables {
    local project_id=$1
    local environment_id=$2
    local service_id=$3

    local variables_query=$(cat <<EOF
query variables {
    variables(
        projectId: $project_id
        environmentId: $environment_id
        serviceId: $service_id
    ) 
}
EOF
)
    local response=$(run_query "$variables_query")
    echo $response
}

# get project id
function get_project_id {
    # todo: error if multiple projects with same name
    local project_name=$1
    project_id=$(echo $projects | jq '.data.projects.edges[] | select(.node.name == "'$project_name'")' | 
    jq '.node.id')
    echo $project_id
}

# get environment id
function get_environment_id {
    local project_name=$1
    local environment_name=$2
    environment_id=$(echo $projects | jq '.data.projects.edges[] | select(.node.name == "'$project_name'")' |
    jq '.node.environments.edges[] | select(.node.name == "'$environment_name'")' |
    jq '.node.id')
    echo $environment_id
}


function get_service_id {
    local service_name=$1
    local project_name=$2
    service_id=$(echo $projects | jq '.data.projects.edges[] | select(.node.name == "'$project_name'")' | 
    jq '.node.services.edges[] | select(.node.name == "'$service_name'")' |
    jq '.node.id')
    echo $service_id
}

function get_variable {
    local key=$1
    local value=$(echo $db_variables | jq '.data.variables["'$key'"]')
    echo $value
}

function run_mutation {
    local mutation=$1
    local response=$(curl -s --request POST https://backboard.railway.app/graphql/v2 \
        -H "Authorization: Bearer $railway_token" \
        -H "Content-Type: application/json" \
        -d "{\"query\": $(echo "$mutation" | jq -R -s -c .)}")
    echo $response
}

function upsert_variable_in_service {
    local service_id=$1
    local key=$2
    local value=$3

    mutation=$(cat <<EOF
mutation variableUpsert {
    variableUpsert(
    input: {
      name: "$key"
      value: $value
      projectId: $project_id    
      environmentId: $environment_id
      serviceId: $service_id
    }
  )
    }
EOF
    )

    local response=$(run_mutation "$mutation")
    echo $response
}

#
# ----- 4. Run Main Script -----
#

echo "Creating Railway project for $project_name..."
if $VERBOSE; then
    railway init -n "$project_name"
    railway up -d
else
    railway init -n "$project_name" > /dev/null
    railway up -d > /dev/null
fi
echo "$project_name created in Railway"
echo ""

echo "Creating Postgres Instance for $project_name..."
if $VERBOSE; then
    railway add -d postgre-sql
else
    railway add -d postgre-sql > /dev/null
fi
# above command already logs once even with redirection to /dev/null 
# echo "Postgres Instance created"
echo ""

# Pause for 5 seconds
echo "Waiting for DB to be ready..."
# todo: fail and retry every 5 seconds, insteasd of assuming the first try will fail
sleep 10
echo "DB should be ready now, resuming..."
echo ""
projects=$(load_projects)

# get project and environment ids
project_id=$(get_project_id $project_name)
environment_id=$(get_environment_id $project_name $environment_name)

# get service ids
db_service_id=$(get_service_id "Postgres" $project_name)
compute_service_id=$(get_service_id $project_name $project_name)

if $VERBOSE; then
    echo "Railway IDs:"
    echo "  Project ID: $project_id"
    echo "  Environment ID: $environment_id"
    echo "  Echo Server Service ID: $compute_service_id"
    echo "  Postgres Service ID: $db_service_id"
fi

echo "Getting environment variables from Postgres..."
# get variables from db
db_variables=$(get_variables $project_id $environment_id $db_service_id)

if $VERBOSE; then
    echo "Postgres Environment Variables:"
    echo $db_variables
fi

# insert variables into compute service
variables=("DATABASE_URL" "DATABASE_PUBLIC_URL")

for key in "${variables[@]}"; do
    value=$(get_variable $key)
    if [ "$value" = "null" ]; then
        echo "Environment variable $key not found for Postgres, skipping"
        continue
    fi
    echo "Found $key=$value, copying to Echo Server..."
    response=$(upsert_variable_in_service $compute_service_id $key $value)
done

echo ""

echo "Copying SESSION_SECRET for Authentication..."
secret=$(openssl rand -base64 32)
response=$(upsert_variable_in_service $compute_service_id "SESSION_SECRET" "\"$secret\"")
echo "Added environment variable SESSION_SECRET=$secret"

if $VERBOSE; then
    echo "Linking service to project:"
    echo "  Project ID: $project_id"
    echo "  Environment ID: $environment_id"
    echo "  Echo Server Service ID: $compute_service_id"
    echo "  Postgres Service ID: $db_service_id (not used for linking)"
fi

echo ""

echo "Attaching Persistent Volume to Echo Server..."
# 'railway link' fails if not run from subshell
export RAILWAY_PROJECT_ID="$project_id"
export RAILWAY_ENVIRONMENT_ID="$environment_id"
export RAILWAY_SERVICE_ID="$compute_service_id"
if $VERBOSE; then
    eval "railway link -p $RAILWAY_PROJECT_ID -e $RAILWAY_ENVIRONMENT_ID -s $RAILWAY_SERVICE_ID"
else
    eval "railway link -p $RAILWAY_PROJECT_ID -e $RAILWAY_ENVIRONMENT_ID -s $RAILWAY_SERVICE_ID" > /dev/null
fi

if $VERBOSE; then
    railway volume add -m /app/public
else
    railway volume add -m /app/public > /dev/null
fi
echo "Persistent Volume attached"
echo ""

echo "Linking domain to project..."
# always show domain (dont hide behind -v)
railway domain
echo ""

echo "Redeploying project..."
if $VERBOSE; then
    railway up -d
else
    railway up -d > /dev/null
fi
echo "Redeployment Successful! Echo Server should be ready to go!"
echo ""

echo "Cleaning up deployment scripts..."
rm -rf "./scripts"
echo "Deployment scripts removed."
echo ""

echo "Completed CI/CD setup in Railway!"
echo ""
echo "!!! IMPORTANT !!!"
echo "For future deployments, use:"
echo ""
echo "  railway up"
echo ""
echo "You should NOT need to run this script again."
echo ""
echo "Happy coding!"
