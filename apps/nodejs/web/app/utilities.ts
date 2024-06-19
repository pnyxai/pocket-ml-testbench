export async function getData() {
  // revalidate to revalidate the data every 60 secs
  return fetch(process.env.API_ENDPOINT_URL!, {next: {revalidate: 60}}).then(res => res.json())
}
