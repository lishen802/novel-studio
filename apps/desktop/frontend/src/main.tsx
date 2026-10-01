import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { AppProvider } from './store/AppContext.tsx';
import App from './App.tsx';
import './index.css';

const router = createBrowserRouter([{ path: '*', element: <AppProvider><App /></AppProvider> }]);
createRoot(document.getElementById('root')!).render(<StrictMode><RouterProvider router={router} /></StrictMode>);
