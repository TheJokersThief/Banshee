#!/bin/bash
set -e

# Copy the tracing workflow template to the workflows directory
cp ${MIGRATION_DIR}/tracing.yml .github/workflows/tracing.yml.tmp

# Create a temporary file to store the names of the existing workflows
tmpDir=$(mktemp -d "${TMPDIR:-/tmp/}$(basename $0).XXXXXXXXXXXX")
workflowFile=${tmpDir}/workflow-names

for file in `find .github/workflows -type f -name "*.yml"`; do

  name=`cat $file | yq '.name' || echo "unparesable yaml"`
  if [ "$name" != "Honeycomb Trace Workflows" ]; then
    echo "      - ${name}" >> ${workflowFile}
  fi
done

export WORKFLOW_NAMES=`cat ${workflowFile} | sort | uniq`
cat .github/workflows/tracing.yml.tmp | envsubst > .github/workflows/tracing.yml
rm .github/workflows/tracing.yml.tmp
