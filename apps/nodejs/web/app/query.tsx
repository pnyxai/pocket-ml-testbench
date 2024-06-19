'use client'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import React from 'react'

export default function ReactQueryProvider({children}: React.PropsWithChildren) {
  return (
    <QueryClientProvider client={new QueryClient()}>
      {children}
    </QueryClientProvider>
  )
}
