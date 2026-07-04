import React from 'react'
import {createRoot} from 'react-dom/client'
import './style.css'
import App from './App'
import {applyTheme, loadTheme} from './theme'

// Apply the saved theme before the first render so the document root already carries the right
// data-theme attribute. Without this the initial paint uses the default theme and the first toggle has
// to reconcile a document that disagrees with React's state.
applyTheme(loadTheme())

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
