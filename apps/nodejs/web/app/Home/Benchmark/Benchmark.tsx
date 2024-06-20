'use client'
import Typography from '@mui/material/Typography'
import { DataGrid, GridColDef, GridRenderCellParams } from '@mui/x-data-grid'
import Stack from '@mui/material/Stack'
import { Button, TextField, useTheme } from '@mui/material'
import { useQuery } from '@tanstack/react-query'
import { getData } from '@/app/utilities'
import { Search } from '@mui/icons-material'
import { useEffect, useRef, useState } from 'react'
import PoktscanLogo from '../../assets/logo/poktscan_logo.svg'


interface RawValues {
  'mean': number,
  'stderr': number
}

interface BenchmarkRawData {
  'average': RawValues,
  'arc': RawValues,
  'hellaswag': RawValues,
  'mmlu': RawValues,
  'truthfulqa': RawValues,
  'winogrande': RawValues,
  'gsm8k': RawValues,
}

interface NodeData {
  qos: {
    response_time: number,
    error_rate: number,
  },
  metrics: BenchmarkRawData
}

type BenchmarkRows = {
  node: string
  response_time: number
  error_rate: number
  average: number
  average_stderr: number
  hellaswag: number
  hellaswag_stderr: number
  arc: number
  arc_stderr: number
  mmlu: number
  mmlu_stderr: number
  truthfulqa: number
  truthfulqa_stderr: number
  winogrande: number
  winogrande_stderr: number
  gsm8k: number
  gsm8k_stderr: number
}

type NodeBenchmarkRawData = Record<string, NodeData>

const getProcessedData = (rawData: NodeBenchmarkRawData, searchText: string) => {
  const rawDataToProcess: NodeBenchmarkRawData = searchText ? searchText.replaceAll(' ', '').split(',').reduce((acc, address) => ({
    ...acc,
    [address]: rawData[address],
  }), {}) : rawData

  const processedData: Array<BenchmarkRows> = []

  for (const node in rawDataToProcess) {
    const nodeData = rawData[node]
    if (!nodeData) continue

    const newRow: Partial<BenchmarkRows> = {
      node,
      error_rate: nodeData.qos.error_rate,
      response_time: nodeData.qos.response_time,
    }

    for (const key in nodeData.metrics) {
      const nodeDataKey = key as keyof BenchmarkRawData

      newRow[nodeDataKey] = nodeData.metrics[nodeDataKey].mean * 100
      newRow[`${nodeDataKey}_stderr`] = nodeData.metrics[nodeDataKey].stderr * 100
    }

    processedData.push(newRow as BenchmarkRows)
  }

  return processedData
}

interface BenchmarkProps {
  initialData: NodeBenchmarkRawData
}

export default function Benchmark({ initialData }: BenchmarkProps) {
  const isLight = useTheme().palette.mode === 'light'
  const { data, isError, refetch } = useQuery<NodeBenchmarkRawData>({
    initialData,
    queryFn: getData,
    queryKey: ['benchmark-data'],
    placeholderData: initialData,
    refetchInterval: 60000,
    refetchOnMount: !initialData,
    refetchOnWindowFocus: false,
    refetchOnReconnect: true,
  })
  const firstMountRef = useRef(false)
  const [searchText, setSearchText] = useState('')
  const [rows, setRows] = useState(getProcessedData(data || initialData, searchText))

  useEffect(() => {
    if (firstMountRef.current) {
      firstMountRef.current = false
      return
    }

    setRows(getProcessedData(data, searchText))
  }, [data, searchText])

  const renderCell = (params: GridRenderCellParams<BenchmarkRows>) => {
    const { row } = params
    const field = params.field as keyof BenchmarkRows

    if (field === 'node') {
      let latencyColor: string
      if (row.response_time <= 300) {
        latencyColor = isLight ? '#3aa624' : '#55e136'
      } else if (row.response_time <= 600) {
        latencyColor = isLight ? '#d57001' : '#e1b936'
      } else {
        latencyColor = isLight ? '#d93030' : '#ff4444'
      }
      return (
        <Stack
          height={1}
          spacing={0.4}
          justifyContent={'center'}
        >
          <Typography fontSize={14}>{row.node}</Typography>
          <Stack direction={'row'} alignItems={'center'} spacing={1.2}>
            <Typography fontSize={12}>
              Latency Average:
              <span
                style={{
                  color: latencyColor,
                  fontWeight: 600,
                  marginLeft: '7px',
                }}
              >
                {row.response_time.toFixed(0)} ms
              </span>
            </Typography>
            <Typography fontSize={12}>
              Success Rate:{' '}
              <span
                style={{
                  fontWeight: 500,
                }}
              >
                {Number(((1 - row.error_rate) * 100).toFixed(1))}%
              </span>
            </Typography>
            <Button
              sx={{
                textTransform: 'none',
                fontSize: 12,
                fontWeight: 700,
                paddingY: 0,
                paddingX: 0.5,
                marginTop: '-2px!important',
                marginLeft: '7px!important',
              }}
              component={'a'} href={`https://poktscan.com/node/${row.node}`}
              target={'_blank'}
            >
              Details
              <Stack height={16} width={16} marginLeft={0.5}>
                <PoktscanLogo />
              </Stack>
            </Button>
          </Stack>
        </Stack>
      )
    }

    const mean = Number(Number(row[field]).toFixed(2))

    if (process.env.SHOW_STDERR !== 'true') {
      return mean
    }

    // @ts-ignore
    const stderr = row[`${field}_stderr`]

    if (stderr) {
      return `${mean} ± ${Number(Number(stderr).toFixed(2))}`
    }

    return mean
  }

  const numberColumnDefaultProps: Partial<GridColDef> = {
    align: 'right', headerAlign: 'right', flex: 1, renderCell, minWidth: 110,
  }

  const columns: Array<GridColDef<BenchmarkRows>> = [{
    field: 'node',
    headerName: 'Node',
    flex: 4,
    minWidth: 360,
    renderCell,
  },
    { field: 'average', headerName: 'Average', ...numberColumnDefaultProps }, {
      field: 'arc', headerName: 'ARC', ...numberColumnDefaultProps,
    }, {
      field: 'hellaswag', headerName: 'HellaSwag', ...numberColumnDefaultProps,
    }, {
      field: 'mmlu',
      headerName: 'MMLU', ...numberColumnDefaultProps,
    }, {
      field: 'truthfulqa',
      headerName: 'TruthfulQA', ...numberColumnDefaultProps,
    }, {
      field: 'winogrande',
      headerName: 'Winogrande', ...numberColumnDefaultProps,
    }, {
      field: 'gsm8k',
      headerName: 'GSM8K', ...numberColumnDefaultProps,
    }]

  return (
    <>
      <Stack
        border={`1px solid ${isLight ? 'rgb(223, 223, 223)' : 'rgb(51, 51, 51)'}`}
        borderRadius={'6px'}
        overflow={'hidden'}
      >
        <Stack
          direction={{ sm: 'row' }}
          spacing={{ xs: 1.5, sm: 2 }}
          alignItems={{ sm: 'center' }}
          justifyContent={'space-between'}
          paddingX={1}
          paddingY={{ xs: 1.5, sm: 'unset' }}
          height={{ xs: 120, sm: 70 }}
          sx={(theme) => ({
            backgroundColor: theme.palette.background.paper,
          })}
          borderBottom={`1px solid ${isLight ? 'rgb(223, 223, 223)' : 'rgb(51, 51, 51)'}`}
        >
          <Typography variant={'h1'} fontSize={26} fontWeight={500}>LLM Benchmark</Typography>
          <TextField
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            size={'small'}
            placeholder={'Search By Node'}
            autoComplete={'off'}
            sx={(theme) => ({
              width: { sm: 270, md: 350 },
              backgroundColor: theme.palette.background.default,
            })}
            InputProps={{
              startAdornment: <Search sx={{ marginLeft: -0.5, marginRight: 0.5 }} />,
            }}
          />
        </Stack>
        <Stack height={'calc(100dvh - 243px)'}>
          {
            isError ? (
              <Stack alignItems={'center'} justifyContent={'center'} flexGrow={1} marginTop={-5}>
                <Typography>There was an error loading the data.</Typography>
                <Button onClick={() => refetch()}>Retry</Button>
              </Stack>
            ) : (
              <DataGrid
                sx={{
                  '--DataGrid-rowBorderColor': isLight ? undefined : '#3a3a3a',
                  border: 'none',
                  borderRadius: 'none',
                  '& .MuiDataGrid-main': {
                    height: 1,
                  },
                  '& .MuiDataGrid-columnHeaderTitle': {
                    userSelect: 'none',
                    fontWeight: 600,
                  },
                  '& .MuiDataGrid-columnHeader, .MuiDataGrid-cell': {
                    outline: 'none!important',
                  },
                  '& .MuiDataGrid-columnSeparator': {
                    display: 'none',
                  },
                }}
                columns={columns}
                rows={rows}
                getRowId={(row) => row.node}
                hideFooterPagination={true}
                hideFooter={true}
                disableColumnFilter={true}
                disableColumnMenu={true}
                disableColumnResize={true}
                disableRowSelectionOnClick={true}
                disableColumnSelector={true}
                rowHeight={60}
                initialState={{
                  sorting: {
                    sortModel: [{ field: 'average', sort: 'desc' }],
                  },
                }}
              />
            )
          }
        </Stack>
      </Stack>
    </>
  )
}
