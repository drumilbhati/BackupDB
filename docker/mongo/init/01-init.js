db = db.getSiblingDB("appdb");

db.users.insertMany([
  { name: "Alice" },
  { name: "Bob" },
  { name: "Charlie" }
]);
