echo "Changed files"
echo "$FILES_CHANGED"
echo "Running execution-plan"
echo "$FILES_CHANGED"  | docker run -i -v $(pwd):/repo eu.gcr.io/freiheit-core/services/execution-plan/execution-plan:0.0.2
