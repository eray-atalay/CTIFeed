import { defineConfig } from 'astro/config';
import preact from '@astrojs/preact';

// https://astro.build/config
export default defineConfig({
  output: 'static',
  build: {
    format: 'file',
    assets: 'assets',
  },
  integrations: [preact()],
  server: {
    port: 4321,
  },
});
