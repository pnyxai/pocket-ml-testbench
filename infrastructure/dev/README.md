### Docker Compose:

WARNING: This setup is exclusively designed to be used for development. 

1. Run `docker compose up -d` to start everything needed to work
2. Create a bash alias to fast access to Temporal CLI inside temporal container
`alias temporal_docker="docker exec temporal-admin-tools temporal"`
3. Create Temporal namespace
`temporal_docker operator namespace create pocket-ml-testbench`

To turn everything down, run: `docker compose down -v` (remove `-v` if you want to keep the data of databases)