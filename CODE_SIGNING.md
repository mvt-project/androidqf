# Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io), certificate by [SignPath Foundation](https://signpath.org).

The Windows executable is signed with SignPath Foundation's certificate, so Windows names SignPath Foundation as the publisher. Only files built from this repository by its release workflow are signed. `androidqf` includes Google's `adb` unmodified; it is not re-signed.

Team roles:

- Committers and reviewers: members of [Amnesty International's Security Lab](https://securitylab.amnesty.org/) with write access to this repository
- Approvers: [Donncha Ó Cearbhaill](https://github.com/DonnchaC)

A team member reviews every change from outside the team before it is merged. An approver approves every release before it is signed.

## Privacy

`androidqf` does not connect to any server on the internet. It talks only to the `adb` server on the same computer, which talks to the phone over USB.
