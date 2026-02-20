# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Primary Reference

See the root `./AGENTS.md` for the main project documentation and guidance.

@/workspace/AGENTS.md

## Additional Component-Specific Guidance

For detailed module-specific implementation guides, check for AGENTS.md files in subdirectories throughout the project.

If you need to ask the user a question, use the tool AskUserQuestion (useful during the clarification phase).

## Updating AGENTS.md Files

When you discover new information that would be helpful for future development work:

- **Update existing AGENTS.md files** when you learn implementation details, debugging insights, or architectural patterns specific to that component
- **Create new AGENTS.md files** in relevant directories when working with areas that don't yet have documentation
- **Add valuable insights** such as common pitfalls, debugging techniques, dependency relationships, or implementation patterns

## Important use subagents liberally

When performing any research concurrent opus subagents can be used for performance and isolation. Use parrallel tool calls and tasks where possible
