db = db.getSiblingDB("url_shortener");

// Create application user with readWrite and dbAdmin roles on the url_shortener database
db.createUser({
    user: process.env.MONGO_APP_USERNAME,
    pwd: process.env.MONGO_APP_PASSWORD,
    roles: [
        {
            role: "readWrite",
            db: "url_shortener",
        },
        {
            role: "dbAdmin",
            db: "url_shortener",
        },
    ],
});
