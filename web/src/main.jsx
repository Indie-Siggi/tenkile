import { render } from 'preact';
import { signal, computed } from '@preact/signals';
import Router from 'preact-router';
import { useEffect } from 'preact/hooks';
import Header from './components/Header.jsx';
import Login from './pages/Login.jsx';
import Dashboard from './pages/Dashboard.jsx';
import Library from './pages/Library.jsx';
import MediaPlayer from './pages/MediaPlayer.jsx';
import { authState, loadAuthFromStorage } from './hooks/useAuth.js';
import './styles/app.css';

// Global auth state signals
export const isAuthenticated = signal(false);
export const currentUser = signal(null);
export const authToken = signal(null);

// Load auth state from localStorage on init
loadAuthFromStorage();

function App() {
  useEffect(() => {
    // Sync auth state from localStorage
    loadAuthFromStorage();
  }, []);

  const handleRouteChange = (e) => {
    const { url } = e;
    // Check if route requires auth
    const publicRoutes = ['/login', '/'];
    const requiresAuth = !publicRoutes.includes(url) && url !== '/login';
    
    if (requiresAuth && !authToken.value) {
      window.location.href = '/login';
    }
  };

  return (
    <div class="app">
      {isAuthenticated.value && <Header />}
      <main class="main-content">
        <Router onChange={handleRouteChange}>
          <Login path="/login" />
          <Dashboard path="/" />
          <Dashboard path="/dashboard" />
          <Library path="/library/:id" />
          <MediaPlayer path="/play/:id" />
        </Router>
      </main>
    </div>
  );
}

render(<App />, document.getElementById('app'));
