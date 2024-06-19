import HomeTabs, {Tabs} from '@/app/Home/Tabs'
import About from '@/app/Home/About'
import MetricsThroughTime from '@/app/Home/MetricsThroughTime'
import Benchmark from '@/app/Home/Benchmark/Benchmark'
import {getData} from '@/app/utilities'

export default async function Home({searchParams}: { searchParams: { tab?: Tabs } }) {
  const data = await getData()

  const about = <About/>
  const benchmark = <Benchmark initialData={data}/>
  const metricsThroughTime = <MetricsThroughTime/>

  return (
    <main>
      <HomeTabs about={about} benchmark={benchmark} metricsThroughTime={metricsThroughTime}
                defaultTab={searchParams.tab}/>
    </main>
  )
}
