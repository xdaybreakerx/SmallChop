const databaseName = process.env.MONGO_DB_NAME || "url_shortener";
db = db.getSiblingDB(databaseName);

// Runs only on fresh storage; match the database used by the application.
db.createUser({
    user: process.env.MONGO_APP_USERNAME,
    pwd: process.env.MONGO_APP_PASSWORD,
    roles: [
        {
            role: "readWrite",
            db: databaseName,
        },
        {
            role: "dbAdmin",
            db: databaseName,
        },
    ],
});
