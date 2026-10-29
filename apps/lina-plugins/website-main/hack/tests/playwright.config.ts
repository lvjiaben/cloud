import {defineConfig} from '@playwright/test';
import {resolve} from 'node:path';
const output=resolve(import.meta.dirname,'../../../../../temp',new Date().toISOString().slice(0,10).replaceAll('-',''));
export default defineConfig({
 testDir:'./e2e',testMatch:'TC*.ts',workers:1,fullyParallel:false,
 outputDir:resolve(output,Date.now()+'-website-test-results'),
 use:{baseURL:process.env.WEBSITE_TEST_URL||'http://127.0.0.1:16680',headless:false,launchOptions:process.env.PLAYWRIGHT_EXECUTABLE_PATH?{executablePath:process.env.PLAYWRIGHT_EXECUTABLE_PATH}:{},viewport:{width:1440,height:1000}},
 reporter:[['list']],
});
