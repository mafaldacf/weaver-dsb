#!/bin/bash

curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=ana&user_id=0&first_name=ana1&last_name=ana2&password=123"
curl -X POST "localhost:9000/wrk2-api/user/register" -d "username=bob&user_id=1&first_name=bob1&last_name=bob2&password=123"
curl -X POST "localhost:9000/wrk2-api/user/follow" -d "user_id=1&followee_id=0"
#curl -X POST "localhost:9000/wrk2-api/post/compose" -d "user_id=0&text=helloworld_0&username=ana&post_type=0"
#curl "localhost:9000/wrk2-api/user-timeline/read" -d "user_id=0"
#curl "localhost:9000/wrk2-api/home-timeline/read" -d "user_id=1"

#curl -X POST "localhost:9000/wrk2-api/user/unfollow" -d "user_id=1&followee_id=0"
