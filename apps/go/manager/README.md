# The Manager App

The manager is in charge of keeping nodes records up to date. It will periodically execute to check on each nodes data
Its main loop will:

0. Check latest height checked
1. Get list of all staked ML nodes' addresses from the Pocket Network if latest height old, if not use mongo.
2. Then, for each node it will:
	1. Check if it already has an entry in the `nodes` collection.
    2. If the node is not in the db, it will trigger all tasks.
	3. Else; it will:
        1. Check for each task sample date
        2. Drop old samples.
        3. Re-calculate task metrics rolling averages.
        3. Trigger new task samples. if no outstanding process exists (use workflow id?)

The trigger process must include limits to avoid clogging the Requester app:
1. Check db for how many are in queue for a given node.
2. Add as many as it can until the given limit using round-robin on metrics.