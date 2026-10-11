// Invoked only inside the suite's fresh, dedicated MongoDB container.
const admin = db.getSiblingDB('admin');
if (!admin.auth(process.env.MONGO_INITDB_ROOT_USERNAME, process.env.MONGO_INITDB_ROOT_PASSWORD)) {
    throw new Error('Benchmark seed authentication failed');
}
const appdb = db.getSiblingDB(process.env.MONGO_DB_NAME);
if (appdb.urls.countDocuments({}) !== 0 || appdb.counters.countDocuments({}) !== 0) {
    throw new Error('Refusing to seed a nonempty database');
}
const rows = JSON.parse(require('fs').readFileSync('/tmp/benchmark-fixtures.json', 'utf8'));
for (let start = 0; start < rows.length; start += 1000) {
    appdb.urls.insertMany(rows.slice(start, start + 1000).map((row) => ({
        _id: NumberLong(String(row.id)),
        longURL: row.destination,
        createdAt: ISODate('2026-01-01T00:00:00Z'),
    })));
}
appdb.counters.insertOne({_id: 'url_counter', seq: NumberLong(String(rows.length))});
print(JSON.stringify({count: appdb.urls.countDocuments({}), indexes: appdb.urls.getIndexes()}));
