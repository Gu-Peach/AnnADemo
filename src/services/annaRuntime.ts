import type { AnnaRuntime } from '../types';

type RuntimeModule = {
  AnnaAppRuntime: {
    connect(): Promise<AnnaRuntime>;
  };
};

export async function connectAnnaRuntime(): Promise<AnnaRuntime> {
  const runtimeModule = (await import(/* @vite-ignore */ '/static/anna-apps/_sdk/latest/index.js')) as RuntimeModule;
  return runtimeModule.AnnaAppRuntime.connect();
}
