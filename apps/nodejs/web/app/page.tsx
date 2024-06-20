import HomeTabs, { Tabs } from '@/app/Home/Tabs'
import About from '@/app/Home/About'
import MetricsThroughTime from '@/app/Home/MetricsThroughTime'
import Benchmark from '@/app/Home/Benchmark/Benchmark'
import { getData } from '@/app/utilities'
import API from '@/app/Home/API'

export default async function Home({ searchParams }: { searchParams: { tab?: Tabs } }) {
  let data

  try {
    data = await getData()
  } catch (e) {
      console.error("unable to get data")
  }

  const about = <About />
  const benchmark = <Benchmark initialData={data} />
  const metricsThroughTime = <MetricsThroughTime />
  const api = <API />

  return (
    <main>
      <HomeTabs
        api={api}
        about={about}
        benchmark={benchmark}
        metricsThroughTime={metricsThroughTime}
        defaultTab={searchParams.tab}
      />
    </main>
  )
}
