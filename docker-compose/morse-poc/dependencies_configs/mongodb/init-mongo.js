try {
    // Fetching the current replica set configuration
    var conf = rs.conf();
    printjson(conf);
} catch (err) {
    print('An error occurred while fetching replica set config: ', err);
    print('Initiating a replica set');

    // Initiating replica set if config is not found (i.e., not a part of any replica set yet)
    rs.initiate(
        {
            _id: "devRs",
            version: 1,
            members: [
                { _id: 0, host: "mongodb:27017" },
            ]
        }
    );
}