'use client'
import Typography from '@mui/material/Typography'
import { DataGrid, GridColDef, GridRenderCellParams } from '@mui/x-data-grid'
import Stack from '@mui/material/Stack'
import { Button, TextField, useTheme } from '@mui/material'
import { useQuery } from '@tanstack/react-query'
import { getData } from '@/app/utilities'
import { Search } from '@mui/icons-material'
import { useEffect, useRef, useState } from 'react'

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

type BenchmarkRows = {
  node: string
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

type NodeBenchmarkRawData = Record<string, BenchmarkRawData>

const getProcessedData = (rawData: NodeBenchmarkRawData, searchText: string) => {
  const rawDataToProcess: NodeBenchmarkRawData = searchText ? searchText.replaceAll(' ', '').split(',').reduce((acc, address) => ({
    ...acc,
    [address]: rawData[address],
  }), {}) : rawData

  const processedData: Array<BenchmarkRows> = []

  for (const node in rawDataToProcess) {
    const newRow: Partial<BenchmarkRows> = {
      node,
    }

    const nodeData = rawData[node]

    if (!nodeData) continue

    for (const key in nodeData) {
      const nodeDataKey = key as keyof BenchmarkRawData

      newRow[nodeDataKey] = nodeData[nodeDataKey].mean * 100
      newRow[`${nodeDataKey}_stderr`] = nodeData[nodeDataKey].stderr * 100
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
    refetchInterval: 60000, refetchOnMount: false, refetchOnWindowFocus: false, refetchOnReconnect: true,
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
      return row[field]
    }

    const mean = Number(Number(row[field]).toFixed(2))
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
    minWidth: 350,
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
