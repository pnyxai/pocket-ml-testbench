# The Manager App

The manager is in charge of keeping suppliers records up to date. It will periodically execute to check on each suppliers data
Its main loop will:

0. Check latest height checked
1. Get list of all staked ML suppliers' addresses from the Pocket Network if latest height old, if not use mongo.
3. Get network parameters, such as blocks per session.
4. Then, for each supplier it will:
	1. Check if it already has an entry in the `suppliers` collection.
    2. If the supplier is not in the db, it will trigger all tasks.
	3. Else; it will:
        1. Check for each task sample date
        2. Drop old samples.
        3. Process new results and delete the tasks from the `tasks` collection (and all others).
        4. Re-calculate task metrics rolling averages.
        5. Trigger new task samples. if no outstanding process exists.

The trigger process must include limits to avoid clogging the Requester app:
1. Check db for how many are in queue for a given supplier.
2. Add as many as it can until the given limit using round-robin on metrics.
3. Trigger tasks periodically
4. Check for tasks requirements (such as having a tokenizer signature or meet a taxonomy result dependency).