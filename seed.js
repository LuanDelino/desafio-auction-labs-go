// Semeia usuarios para testar a API: nao existe endpoint que crie usuario, e
// GET /user/:userId sem esses registros so responde 404.
// Uso: docker exec -i mongodb mongosh --quiet -u admin -p admin \
//        --authenticationDatabase admin auctions --file /dev/stdin < seed.js

const users = [
  { _id: "8b3f6f1a-1c2d-4e5f-9a70-111111111111", name: "Ana Souza" },
  { _id: "9c4a7e2b-2d3e-4f60-8b81-222222222222", name: "Bruno Lima" },
];

for (const user of users) {
  db.users.updateOne({ _id: user._id }, { $set: { name: user.name } }, { upsert: true });
}

print("usuarios disponiveis:");
db.users.find({}, { _id: 1, name: 1 }).forEach((u) => print("  " + u._id + "  " + u.name));
