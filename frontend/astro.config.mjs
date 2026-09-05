import { defineConfig } from 'astro/config';

// https://astro.build/config
export default defineConfig({
  output: 'static',
  build: {
    format: 'file',
  },
  server: {
    port: 4321,
  },
});
