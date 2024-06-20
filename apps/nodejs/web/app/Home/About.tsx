import Typography from '@mui/material/Typography'
import Stack from '@mui/material/Stack'

export default function About() {
  return (
    <Stack sx={{
      '& p': {
        marginTop: 1,
      },
      'p, a': {
        fontSize: 14,
      },
      '& h2, h3': {
        marginTop: 2,
      },
      '& a': {
        textDecoration: 'none',
        color: '#4379ff',
        fontWeight: 600,
        '&:hover': {
          textDecoration: 'underline',
        },
      },
    }}>
      <Typography variant={'h1'} fontSize={26} fontWeight={500}>About</Typography>
      <Typography>
        Evaluating a language model (LM) is a complex task that involves analyzing many different aspect of its
        capabilities: from recall to solving math models. An effort to simplify these tasks was the creation of
        leaderboards such as the{' '}
        <a
          href={'https://huggingface.co/spaces/open-llm-leaderboard/open_llm_leaderboard'}
          target={'_blank'}
        >
          Open LLM Leaderboard
        </a> by HuggingFace.
      </Typography>
      <Typography>
        This leaderboard is an effort to provide the POKT Network users with the same information that they are used to
        look at when choosing an open LLM, but with the following advantages:
      </Typography>
      <ul>
        <li>
          <Typography>
            <strong>What you see is what you get:</strong> You are not looking at model names, you are looking at actual
            inference endpoints.
          </Typography>
        </li>
        <li>
          <Typography>
            <strong>Live Data:</strong> We run these tests 24-7, the scores are updated each time the inference node
            enters in session.
          </Typography>
        </li>
        <li>
          <Typography>
            <strong>Trustless and Permissionless:</strong> If you connect your model to the POKT Network, we will track
            it, we don't care who is behind the node or what they claim about it. We test and report, period.
          </Typography>
        </li>
      </ul>
      <Typography variant={'h3'} fontSize={20} fontWeight={500}>Tasks</Typography>
      <Typography>
        Following Hugging Face's Open LLM Leaderboard, we evaluate models on 6 key benchmarks using our Machine Learning
        Test Bench that implements the Eleuther AI Language Model Evaluation Harness under the hood. The tasks
        implemented are:
      </Typography>
      <ul>
        <li>
          <Typography>
            <a href={'https://arxiv.org/abs/1803.05457'} target={'_blank'}>AI2 Reasoning Challenge</a> (25-shot) - a set
            of grade-school science
            questions.
          </Typography>
        </li>
        <li>
          <Typography>
            <a href={'https://arxiv.org/abs/1905.07830'} target={'_blank'}>HellaSwag</a> (10-shot) - a test of
            commonsense inference, which is easy for
            humans (~95%) but challenging for SOTA models.
          </Typography>
        </li>
        <li>
          <Typography>
            <a href={'https://arxiv.org/abs/2009.03300'} target={'_blank'}>MMLU</a> (5-shot) - a test to measure a text
            model's multitask accuracy. The
            test covers 57 tasks including elementary mathematics, US history, computer science, law, and more.
          </Typography>
        </li>
        <li>
          <Typography>
            <a href={'https://arxiv.org/abs/2109.07958'} target={'_blank'}>TruthfulQA</a> (0-shot) - a test to measure a
            model's propensity to reproduce
            falsehoods commonly found online. Note: TruthfulQA is technically a 6-shot task in the Harness because each
            example is prepended with 6 Q/A pairs, even in the 0-shot setting.
          </Typography>
        </li>
        <li>
          <Typography>
            <a href={'https://arxiv.org/abs/1907.10641'} target={'_blank'}>Winogrande</a> (5-shot) - an adversarial and
            difficult Winograd benchmark at
            scale, for commonsense reasoning.
          </Typography>
        </li>
        <li>
          <Typography>
            <a href={'https://arxiv.org/abs/2110.14168'} target={'_blank'}>GSM8k</a> (5-shot) - diverse grade school
            math word problems to measure a
            model's ability to solve multi-step mathematical reasoning problems (only partial match).
          </Typography>
        </li>
      </ul>
      <Typography>
        Since we are dealing with live endpoints, we do not sample the whole datasets, instead we sample 50 samples for
        each task (or sub-task in the case of MMLU). The effect on the tests accuracy is less than <code>5%</code> (<a
        href={'http://arxiv.org/abs/2402.14992'} target={'_blank'}>tinyBenchmarks,
        Polo et al.</a>).
      </Typography>

      <Typography>Remember, a higher score is a better score!</Typography>

      <Typography variant={'h3'} fontSize={20} fontWeight={500}>Reproducibility</Typography>
      <Typography>
        The code used to produce these results is completely available in our repository <a
        href={'https://github.com/pokt-scan/pocket-ml-testbench'} target={'_blank'}>Machine Learning Test Bench</a>.
        Remember that the results will not be numerically exact as they depend on a random sample of the complete
        dataset and also the available nodes may vary their performance with time.
        <br />
        You will also need a POKT Network App to connect to the network and consume relays, this will have a cost to be
        paid in POKT.
      </Typography>
    </Stack>
  )
}
