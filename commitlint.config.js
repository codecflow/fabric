module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    // only these types allowed
    'type-enum': [
      2, 'always',
      [
        'feat',
        'fix',
        'perf',
        'refactor',
        'test',
        'revert',
        'chore',
        'docs',
        'content',
        'build',
        'ci',
        'hotfix',
        'bugfix',
        'release'
      ]
    ],
    // require a scope (optional—remove if you don’t want to enforce)
    'scope-empty': [2, 'never'],
    // allow any subject-case
    'subject-case': [0]
  }
};
