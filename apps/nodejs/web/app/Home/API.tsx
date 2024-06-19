import Typography from '@mui/material/Typography'

export default function API() {
  return (
    <>
      <Typography variant={'h2'} fontSize={23} fontWeight={500}>API</Typography>
      <Typography fontSize={14} marginTop={1}>
        Want to use this data live to feed your App or POKT Network portal?
        <br />
        We got you, this will be deployed with an API to query the inference node's performance.
      </Typography>
    </>
  )
}
