import './assets/index.scss';
import React from 'react';
import { createRoot } from 'react-dom/client';
import { Routes } from './Routes';
import { BrowserRouter } from 'react-router-dom';

const container = document.getElementById('root');
const root = createRoot(container!); // createRoot(container!) using TypeScript
root.render(
    <React.StrictMode>
        <BrowserRouter>
            <Routes />
        </BrowserRouter>
    </React.StrictMode>
);
