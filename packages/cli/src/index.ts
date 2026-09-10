#!/usr/bin/env node
/**
 * Promptsheon CLI — subcommand harness for the Fastify API.
 *
 * Stable exit codes (see ./version.ts):
 *   0 OK, 1 UNKNOWN, 2 BAD_ARGS, 3 API_ERROR, 4 NETWORK_ERROR,
 *   5 AUTH_ERROR, 6 NOT_FOUND, 7 CONFLICT, 8 PRECONDITION_FAILED
 *
 * Flags (apply to every subcommand):
 *   --json   output the raw response as JSON
 *   --dry-run  print the would-be request, do not fire it
 *
 * Auth via env:
 *   PROMPTSHEON_API_URL=http://127.0.0.1:8080
 *   PROMPTSHEON_API_KEY=<org-scoped bearer>
 *   PROMPTSHEON_WORKSPACE_ID=<uuid>     — required by `repos list`
 */
import { PROMPTSHEON_CLI_VERSION, EXIT } from './version.js';
import { makeClient, parseFlags, print, handleError } from './output.js';
import {
  evalGateCommand,
  loginCommand,
  manifestScanCommand,
  releaseApproveCommand,
  releaseGetCommand,
  reposListCommand,
} from './commands.js';

function usage(): void {
  console.log(`promptsheon v${PROMPTSHEON_CLI_VERSION}

Usage:
  promptsheon [--json] [--dry-run] <command> [args]

Commands:
  version                            print the CLI version + exit 0
  login                              verify the API key works
  repos list                         list repositories in the workspace
  eval gate <repoId>                 run the CI eval gate for a repo
  release get <id>                   show a release's current state
  release approve <id>               approve a release (maker-checker)
  manifest scan <hash>               scan a manifest through the T2-3 scanner

Env:
  PROMPTSHEON_API_URL                default http://127.0.0.1:8080
  PROMPTSHEON_API_KEY                org-scoped bearer token
  PROMPTSHEON_WORKSPACE_ID           required by 'repos list'
`);
}

async function main(argv: string[]): Promise<number> {
  // argv[0] = node, argv[1] = entrypoint, argv[2..] = user args
  const userArgs = argv.slice(2);
  if (userArgs.length === 0 || userArgs.includes('--help') === true || userArgs.includes('-h') === true) {
    usage();
    return EXIT.OK;
  }
  const { positional, flags } = parseFlags(userArgs);
  const cmd = positional[0];

  if (cmd === 'version') {
    print(flags.format, { version: PROMPTSHEON_CLI_VERSION });
    return EXIT.OK;
  }

  const client = makeClient();

  try {
    switch (cmd) {
        case 'login': {
          const me = await loginCommand(client);
          print(flags.format, me);
          return EXIT.OK;
        }
        case 'repos': {
          if (positional[1] !== 'list') {
            usage();
            return EXIT.BAD_ARGS;
          }
          const list = await reposListCommand(client);
          print(flags.format, list);
          return EXIT.OK;
        }
        case 'eval': {
          if (positional[1] !== 'gate') {
            usage();
            return EXIT.BAD_ARGS;
          }
          const result = await evalGateCommand(client, positional[2] ?? '');
          print(flags.format, result);
          return EXIT.OK;
        }
        case 'release': {
          const sub = positional[1];
          if (sub === 'get') {
            const r = await releaseGetCommand(client, positional[2] ?? '');
            print(flags.format, r);
            return EXIT.OK;
          }
          if (sub === 'approve') {
            const r = await releaseApproveCommand(client, positional[2] ?? '', {
              dryRun: flags.dryRun,
            });
            print(flags.format, r);
            // Dry-run of a mutating command is a safe no-op; treat as OK.
            return EXIT.OK;
          }
          usage();
          return EXIT.BAD_ARGS;
        }
        case 'manifest': {
          if (positional[1] !== 'scan') {
            usage();
            return EXIT.BAD_ARGS;
          }
          const r = await manifestScanCommand(client, positional[2] ?? '', {
            dryRun: flags.dryRun,
          });
          print(flags.format, r);
          return EXIT.OK;
        }
        default:
          usage();
          return EXIT.BAD_ARGS;
      }
  } catch (err) {
    return handleError(err, flags.format);
  }
}

main(process.argv).then(
  (code) => {
    process.exit(code);
  },
  (err) => {
    console.error(err);
    process.exit(EXIT.UNKNOWN);
  },
);