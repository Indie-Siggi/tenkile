import { render } from 'preact';
import Router from 'preact-router';
import Header from './components/Header.jsx';
import Login from './pages/Login.jsx';
import Dashboard from './pages/Dashboard.jsx';
import Library from './pages/Library.jsx';
import MediaPlayer from './pages/MediaPlayer.jsx';
import { authState, loadAuthFromStorage } from './hooks/useAuth.js';
import './styles/app.css';

// Load auth state from sessionStorage on init (one-time)
loadAuthFromStorage();

/** Guard component: renders children only if authenticated, otherwise redirects to login. */
function ProtectedRoute({ component: Component, ...props }) {
  if (!authState.isAuthenticated.value) {
    // Use replaceState to avoid polluting browser history
    if (typeof window !== 'undefined' && window.location.pathname !== '/login') {
      window.location.href = '/login';
    }
    return null;
  }
  return <Component {...props} />;
}

function App() {
  return (
    <div class="app">
      {authState.isAuthenticated.value && <Header />}
      <main class="main-content">
        <Router>
          <Login path="/login" />
          <ProtectedRoute path="/" component={Dashboard} />
          <ProtectedRoute path="/dashboard" component={Dashboard} />
          <ProtectedRoute path="/library/:id" component={Library} />
          <ProtectedRoute path="/play/:id" component={MediaPlayer} />
        </Router>
      </main>
    </div>
  );
}

render(<App />, document.getElementById('app'));
