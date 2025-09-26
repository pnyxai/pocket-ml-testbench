import random

RANDOM_SEED = 42
RANDOM_STUFF = """
Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna 
aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. 
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint 
occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.
"""


def prompt_setup():
    random.seed(RANDOM_SEED)

def get_prompt():
    rnd_strs = random.choices(RANDOM_STUFF.replace("\n", "").split(" "),k=10)
    return " ".join(rnd_strs)+" "