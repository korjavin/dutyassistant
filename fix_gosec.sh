#!/bin/bash
sed -i 's/return maxQueueUsers\[rand.IntN(len(maxQueueUsers))\]/\/* #nosec G404 *\/ return maxQueueUsers\[rand.IntN(len(maxQueueUsers))\]/g' internal/scheduler/scheduler.go
sed -i 's/return users\[rand.IntN(len(users))\]/\/* #nosec G404 *\/ return users\[rand.IntN(len(users))\]/g' internal/scheduler/scheduler.go
sed -i 's/return candidateUsers\[rand.IntN(len(candidateUsers))\]/\/* #nosec G404 *\/ return candidateUsers\[rand.IntN(len(candidateUsers))\]/g' internal/scheduler/scheduler.go
