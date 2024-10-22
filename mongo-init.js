db = db.getSiblingDB(process.env.MONGO_DB_NAME);

// Create application user with readWrite role on the url_shortener database
db.createUser({
    user: process.env.MONGO_APP_USERNAME,
    pwd: process.env.MONGO_APP_PASSWORD,
    roles: [
        {
            role: "readWrite",
            db: process.env.MONGO_DB_NAME,
        },
    ],
});
