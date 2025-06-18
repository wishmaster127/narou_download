import './App.css';
import NarouDownload from './pages/NarouDownload';
import { MantineProvider } from '@mantine/core';

function App() {
    return (
        <MantineProvider>
            <div id="app">
                <NarouDownload />
            </div>
        </MantineProvider>
    )
}

export default App
