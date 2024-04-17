rs.initiate(
    {
        _id: "devRs",
        version: 1,
        members: [
            { _id: 0, host : "localhost:27017" },
        ]
    }
)