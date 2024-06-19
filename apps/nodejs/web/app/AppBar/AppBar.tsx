import MUIAppBar from '@mui/material/AppBar'
import Toolbar from '@mui/material/Toolbar'
import Stack from '@mui/material/Stack'
import React from 'react'
import Logo from '../assets/logo/logo.svg'
import ToggleThemeButton from '@/app/AppBar/ToggleTheme'

const height = '60px!important'
export default function AppBar() {
  return (
    <>
      <MUIAppBar
        elevation={0}
        sx={{
          height,
          paddingX: 1,
        }}
      >
        <Toolbar
          sx={{
            height,
            minHeight: height,
            width: '100%',
            alignItems: 'center',
            justifyContent: 'space-between',
            paddingX: { xs: '5px!important', sm: '10px!important', xl: '20px!important' },
          }}
        >
          <Stack
            marginLeft={-1}
            alignItems={'center'}
            justifyContent={'center'}
            height={{ xs: 30, sm: 40, md: 50 }}
            width={{ xs: 70, sm: 100, md: 120 }}
          >
            <Logo className={'logo'} viewBox={'0 0 200 84'} />
          </Stack>
          <ToggleThemeButton />
        </Toolbar>
      </MUIAppBar>
      <Toolbar sx={{
        height,
        minHeight: height,
      }} />
    </>
  )
}
