// Retrieve environment variables
const userName = process.env.MONGO_INITDB_ROOT_USERNAME;
const password = process.env.MONGO_INITDB_ROOT_PASSWORD;
const dbName = process.env.MONGO_INITDB_DATABASE;

// Check if required environment variables are set
if (!userName || !password || !dbName) {
    console.error("Please set MONGO_INITDB_ROOT_USERNAME, MONGO_INITDB_ROOT_PASSWORD, and MONGO_INITDB_DATABASE environment variables.");
    process.exit(1); // Exit with error code
}

// Create user with retrieved information
db.createUser(
    {
        user: userName,
        pwd: password,
        roles: [
            {
                role: "readWrite",
                db: dbName
            }
        ]
    }
);

// Create collection "test" in database dbName 
db.createCollection("tasks");
db.createCollection("instances");
db.createCollection("prompts");
db.createCollection("responses");