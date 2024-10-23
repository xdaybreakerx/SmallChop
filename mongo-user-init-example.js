db = db.getSiblingDB("example-db");

// Example user setup
db.createUser({
    user: "example-user",
    pwd: "example-password",
    roles: [
        {
            role: "readWrite",
            db: "example-db",
        },
    ],
});