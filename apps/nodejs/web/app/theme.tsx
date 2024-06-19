'use client'
import {Open_Sans} from 'next/font/google'
import {createTheme, Theme} from '@mui/material/styles'
import {ThemeProvider as MuiThemeProvider} from '@mui/material/styles'
import React, {useEffect, useState} from 'react'

const openSans = Open_Sans({
  weight: ['300', '400', '500', '600', '700', '800'],
  subsets: ['latin'],
  display: 'swap',
})

const defaultTheme = createTheme({
  typography: {
    fontFamily: openSans.style.fontFamily,
  },
  spacing: 10,
  palette: {
    text: {},
  },
  components: {

    MuiAppBar: {
      defaultProps: {
        color: 'default',
      },
      styleOverrides: {
        root: ({theme}) => ({
          backgroundColor: theme.palette.background.paper,
          backgroundImage: 'none',
        }),
      },
    },
  },
})

//todo: create color constant
const lightTheme = createTheme(defaultTheme, {
  palette: {
    mode: 'light',
    background: {
      paper: '#fbfaff',
      default: '#fff',
    },
  },
  components: {
    MuiSvgIcon: {
      styleOverrides: {
        root: {
          color: '#454545de',
        },
      },
    },
    MuiAppBar: {
      styleOverrides: {
        root: {
          boxShadow: 'rgba(0, 0, 0, 0.05) 0px 4px 18px 0px',
          borderBottom: '1px solid rgb(223, 223, 223) !important',
        },
      },
    },
  },
})

const darkTheme = createTheme(defaultTheme, {
  palette: {
    mode: 'dark',
    background: {
      default: '#111111',
      paper: '#1a1a1a',
    },
    text: {
      primary: 'white',
    },
  },
  components: {
    MuiTab: {
      styleOverrides: {
        root: {
          color: '#c5c5c5',
        },
      },
    },
    MuiSvgIcon: {
      styleOverrides: {
        root: {
          color: 'rgba(213,213,213,0.87)',
        },
      },
    },
    MuiAppBar: {
      styleOverrides: {
        root: {
          borderBottom: '1px solid rgb(51, 51, 51) !important',
        },
      },
    },
    MuiToolbar: {
      styleOverrides: {
        root: {
          '& .logo_svg__logo-letter': {
            fill: 'white',
          },
        },
      },
    },
  },
})

export function setCookie(name: string, value: string, days?: number) {
  let expires = ''
  if (days) {
    const date = new Date()
    date.setTime(date.getTime() + days * 24 * 60 * 60 * 1000)
    expires = '; expires=' + date.toUTCString()
  }
  document.cookie = name + '=' + (value || '') + expires + '; path=/'
}

export default function ThemeProvider({children, defaultTheme}: React.PropsWithChildren<{
  defaultTheme: string
}>) {
  const [muiTheme, setMuiTheme] = useState<Theme>(defaultTheme === 'light' ? lightTheme : darkTheme)

  useEffect(() => {
    const listener = (event: CustomEvent) => {
      console.log('event received', event.detail)
      setMuiTheme(event.detail === 'light' ? lightTheme : darkTheme)
      setCookie('theme', event.detail)
    }

    // @ts-ignore
    window.addEventListener('themeChanged', listener)

    return () => {
      // @ts-ignore
      window.removeEventListener('themeChanged', listener)
    }
  }, [])

  useEffect(() => {
    console.log('theme changed:', muiTheme.palette.mode)
  }, [muiTheme])

  return (
    <MuiThemeProvider theme={muiTheme}>
      {children}
    </MuiThemeProvider>
  )
}
