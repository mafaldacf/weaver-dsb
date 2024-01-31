#!/bin/bash

curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=user_0&user_id=0&first_name=m&last_name=f&password=123"
curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=user_1&user_id=1&first_name=m&last_name=f&password=123"
curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_id=1&followee_id=0"
curl -X POST "localhost:9000/wrk2-api/post/compose" -d "user_id=0&text=helloworld&username=user_0&post_type=0"
curl "localhost:9000/wrk2-api/user-timeline/read" -d "user_id=0"
