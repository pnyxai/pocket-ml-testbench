'use client'

import {useTheme} from '@mui/material'
import IconButton from '@mui/material/IconButton'
import DarkThemeIcon from './assets/icons/dark_icon_theme.svg'
import LightThemeIcon from './assets/icons/light_icon_theme.svg'

export default function ToggleThemeButton() {
  const isLight = useTheme().palette.mode === 'light'

  const toggleTheme = () => {
    const newTheme = !isLight ? 'light' : 'dark'
    window.dispatchEvent(new CustomEvent('themeChanged', {detail: newTheme}))
  }

  return (
    <IconButton
      sx={{
        padding: 0,
        height: 40,
        width: 40,
      }}
      onClick={toggleTheme}
    >
      {isLight ? (
        <DarkThemeIcon viewBox="0 0 40 40"/>
      ) : (
        <LightThemeIcon viewBox="0 0 40 40"/>
      )}
    </IconButton>
  )
}
