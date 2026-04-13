---
name: atlassian
description: Delegates Atlassian operations (Jira + Confluence) to a sub-agent
user-invocable: false
---

# Atlassian Integration

Delegates Jira and Confluence operations to a specialized sub-agent.

## Capabilities

### Jira

- Search tasks: `searchJiraIssuesUsingJql`
- Create issues: `createJiraIssue`
- Update issues: `editJiraIssue`
- Transition issues: `transitionJiraIssue`
- Add comments: `addCommentToJiraIssue`
- Register worklogs: `addWorklogToJiraIssue`

### Confluence

- Search pages: `searchConfluenceUsingCql`
- Read pages: `getConfluencePage`
- Create pages: `createConfluencePage`
- Update pages: `updateConfluencePage`

## Context Inference

When on a branch like `fix/PROJ-123-description`:

- Extract issue key: `PROJ-123`
- Auto-fetch issue context before starting work
- Link commits and PRs to the issue
