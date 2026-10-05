# Copilot Instructions

- Before writing code, write a plan of what needs to be done into the `docs` directory.
- Whenever something is unclear, ask the user for clarification.
- When evaluating options, focus on downsides and caveats.
- Only proceed with implementation when requirements are clear.
- Do not start with implementation without confirmation of the user.
- Follow the plan when writing code.
- Keep code short and concise.
- Follow SOLID principles amd write DRY code within reason.
- Develop test-driven:
  1. List scenarios for the new feature
  2. Write a test for an item on the list
  3. Run all tests. The new test should fail – for expected reasons
  4. Write the simplest code that passes the new test
  5. All tests should now pass
  6. Refactor as needed while ensuring all tests continue to pass
  7. Repeat
- and ensure that your tests cover relevant edge cases instead of just
  the happy path.
- If possible and available, use IDE built-in functionality over executing terminal commands.
- If it exists, keep the `README.md` up-to-date.
  It should be concise and tell developers and users about the project and common workflows.
- Do not get lost in details when writing documentation (e.g. `README.md`, `docs/**`),
  but do correct outdated or wrong information.
