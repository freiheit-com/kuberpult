git diff main --name-only | docker run -i -v $(pwd):/repo eu.gcr.io/freiheit-core/services/execution-plan/execution-plan:0.0.1
