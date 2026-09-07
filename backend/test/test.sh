curl -s -X POST localhost:8000/api/shoots/abc123/archive
# {"job_id":"job_abc123"}

curl -s localhost:8000/api/jobs/job_abc123
# {"id":"job_abc123","state":"running","done":3,"total":42}

curl -N localhost:8000/api/jobs/job_abc123/events
# streams progress lines every ~200ms until done