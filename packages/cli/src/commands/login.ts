import { Command } from "commander";
import { saveConfig } from "../config.js";

export function registerLoginCommand(program: Command) {
  program
    .command("login")
    .description("Authenticate Prism CLI with API key and gateway URL")
    .option("-k, --key <apiKey>", "Gateway API Key (prism_sk_*)")
    .option("-u, --url <url>", "Base API Gateway URL", "http://localhost:8080")
    .action((options) => {
      const apiKey = options.key;
      const baseURL = options.url;

      if (!apiKey) {
        console.error("Error: --key is required. Usage: prism login --key prism_sk_xxx");
        process.exit(1);
      }

      saveConfig({ apiKey, baseURL });
      console.log(`Successfully authenticated! Configuration saved to ~/.prism/config.json`);
      console.log(`Base URL: ${baseURL}`);
    });
}

