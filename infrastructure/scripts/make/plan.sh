echo "changedfileenv"
echo "$CHANGED_FILES"
cat .changedFiles  | docker run -i -v $(pwd):/repo eu.gcr.io/freiheit-core/services/execution-plan/execution-plan:0.0.2
