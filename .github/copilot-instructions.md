# Copilot Instructions

- Before writing code, write a plan of what needs to be done into the `docs` directory.
- Whenever something is unclear, ask the user for clarification.
- When evaluating options, focus on downsides and caveats.
- Only proceed with implementation when requirements are clear.
- Do not start with implementation without confirmation of the user.
- Follow the plan when writing code.
- Keep code short and concise.
- Follow SOLID principles amd write DRY code within reason.
- Use library functions whenever possible to reduce not-invented-here syndrome code.
- Develop test-driven:
  1. List scenarios for the new feature
  2. Write a test for one item on the list
  3. Run all tests. The new test should fail – for expected reasons
  4. Write the simplest code that passes the new test, while keeping the final intention of the feature to implement in mind
  5. All tests should now pass
  6. Refactor as needed while ensuring all tests continue to pass
  7. Repeat
- Ensure that tests cover both the happy path and all relevant edge cases.
- If possible and available, use IDE built-in functionality or MCP server over executing terminal commands.
- If it exists, keep the `README.md` up-to-date.
  It should be concise and tell developers and users about the project and common workflows.
  It is not meant for detailed documentation about architecture, decisions or code details.
- Do not get lost in details when writing documentation (e.g. `README.md`, `docs/**`),
  but do correct outdated or wrong information.
