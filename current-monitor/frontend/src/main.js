import { mount } from 'svelte';
import App from './App.svelte';

// Mount the App component natively using the new Svelte 5 standard API
const app = mount(App, {
  target: document.getElementById('app'),
});

export default app;

