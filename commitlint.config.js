module.exports = {
  rules: {
      "body-leading-blank": [2, "always", ''],
      "footer-leading-blank": [0, "always"],
      "header-min-length": [2, "always", 10],
      "subject-empty": [2, "never"],
      "subject-min-length": [2, "always", 10],
      "type-enum": [2, "always", ['fix', 'feat', 'chore', 'revert']],
      "type-empty": [2, "never"],
  
      "body-max-line-length": [1, "never", 80],
      "header-max-length": [1, "always", 120],
      "subject-max-length": [1, "always", 120],
  
      "body-full-stop": [0, "always", ''],
      "body-empty": [0, "never", ''],
      "body-min-length": [0, "always", 0],
      "body-case": [0, "always", 'lower-case'],
      "footer-max-length": [0, "always", 0],
      "footer-max-line-length": [1, "never", 80],
      "footer-min-length": [0, "always", 0],
      "header-case": [0, "always", 'lower-case'],
      "header-full-stop": [0, "never", ''],
      "scope-enum": [0, "always", ["", "CI", "deps"]],
      "scope-case": [0, "always", 'lower-case'],
      "scope-empty": [0, "never"],
      "scope-max-length": [1, "always", 10],
      "scope-min-length": [0, "always", 2],
      "subject-case": [0, "always", 'lower-case'],
      "subject-full-stop": [0, "never", ''],
      "subject-exclamation-mark": [0, "never"],
      "type-case": [0, "always", 'lower-case'],
      "type-max-length": [0, "always", 120],
      "type-min-length": [0, "always", 0],
      "signed-off-by": [0, "always", 'Signed-off-by:'],
      "trailer-exists": [0, "always", 'Signed-off-by:'],
      "check-story-reference": [2, "always"],
  },
  plugins: [
    {
      rules: {
        'check-story-reference': ({body}) => {
          if (body === null) {
            return [false, 'Commit message must contain a valid reference to a story'];
          }
          const REF_PATTERN = /Ref: SRX-[A-Z0-9]{6}/;
  	return [
  	  REF_PATTERN.test(body),
  	  'Commit message must contain a valid reference to a story',
  	];
        },
      },
    },
  ],
};
