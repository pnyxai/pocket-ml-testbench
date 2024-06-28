# Leaderboard Site

This is a proof of concept for a leaderboard site (outside the scope of the [original socket](https://forum.pokt.network/t/open-pokt-ai-lab-socket/5056)). It is a minimalist site built with [Next.js](https://nextjs.org/) and consuming another minimalist [MLTB API](../../python/api/README.md).

### Setting-Up

Add a .env.local file with:

- API_ENDPOINT_URL: this should be the url where the data of the table is going to be fetched. For
  example: http://localhost:3001/leaderboard
- SHOW_STDERR: to enable the standard deviation use this env with "true"

Run the server in development mode with:

```bash
pnpm dev
```

Build the server in production mode with:

```bash
pnpm build
```

Start the server built with:

```bash
pnpm start
```

