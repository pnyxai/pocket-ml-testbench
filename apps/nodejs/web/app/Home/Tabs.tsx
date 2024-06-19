'use client'
import Box from '@mui/material/Box'
import Tab from '@mui/material/Tab'
import TabContext from '@mui/lab/TabContext'
import TabList from '@mui/lab/TabList'
import TabPanel from '@mui/lab/TabPanel'
import React, { useState } from 'react'
import { useTheme } from '@mui/material'

interface HomeTabsProps {
  about: React.ReactNode
  benchmark: React.ReactNode
  metricsThroughTime: React.ReactNode
  defaultTab?: Tabs
}

export enum Tabs {
  about = 'about',
  benchmark = 'benchmark',
  metricsThroughTime = 'metricsThroughTime',
}

export default function HomeTabs({ about, defaultTab, benchmark, metricsThroughTime }: HomeTabsProps) {
  const isLight = useTheme().palette.mode === 'light'
  const [tab, setTab] = useState<Tabs>(Object.values(Tabs).includes(defaultTab!) ? defaultTab! : Tabs.benchmark)

  const handleChange = (event: React.SyntheticEvent, newValue: Tabs) => {
    setTab(newValue)
  }

  return (
    <Box sx={{ width: '100%', '& .MuiTabPanel-root': { padding: 2, paddingBottom: '0!important' } }} padding={2}>
      <TabContext value={tab}>
        <Box sx={{ borderBottom: `1px solid ${isLight ? 'rgb(223, 223, 223)' : 'rgb(51, 51, 51)'}` }}>
          <TabList onChange={handleChange}>
            <Tab label={'LLM Benchmark'} value={Tabs.benchmark} />
            <Tab label={'Metrics Through Time'} value={Tabs.metricsThroughTime} />
            <Tab label={'About'} value={Tabs.about} />
          </TabList>
        </Box>
        <Box
          className={'MuiTabPanel-root'}
          visibility={tab === Tabs.benchmark ? 'visible' : 'hidden'}
          sx={{
            opacity: tab === Tabs.benchmark ? 1 : 0,
            pointerEvents: tab === Tabs.benchmark ? undefined : 'none',
          }}
          height={tab === Tabs.benchmark ? undefined : 0}
        >
          {benchmark}
        </Box>
        <TabPanel value={Tabs.metricsThroughTime}>{metricsThroughTime}</TabPanel>
        <TabPanel value={Tabs.about}>{about}</TabPanel>
      </TabContext>
    </Box>
  )

}
