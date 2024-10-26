#!/bin/bash

# Replace placeholders in js file with actual environment variable values
sed -i "s/MONGO_APP_USERNAME/${MONGO_APP_USERNAME}/g" /docker-entrypoint-initdb.d/mongo-user-init.js
sed -i "s/MONGO_APP_PASSWORD/${MONGO_APP_PASSWORD}/g" /docker-entrypoint-initdb.d/mongo-user-init.js

# Start MongoDB with the modified init script
exec "$@"