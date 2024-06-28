# The Requester App (Morse - V0)

The `Requester` App will process any task that is in the `tasks` collection and has field `done=false`. 
It will pick the task, recontruct the prompt, path and target node address and if the node is within a session it will send the relay.
After the relay is done it will withe the raw response to the `responses` collection. 

The Requester app will handle all expected situations with a permissions node:
- No answer.
- Refuse to work (due to proof sealed or out of session).
- Respond correctly with a 4xx answer.
The Requester will only retry the relay when it is correct to do so.
