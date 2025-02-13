/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/
import { EnvironmentChip, EnvironmentChipProps, EnvironmentGroupChip } from './EnvironmentGroupChip';
import { fireEvent, render } from '@testing-library/react';
import { Environment, EnvironmentGroup, Lock, Priority, GetAllEnvTeamLocksResponse } from '../../../api/api';
import { EnvironmentGroupExtended, UpdateOverview, updateAllEnvLocks } from '../../utils/store';
import { Spy } from 'spy4js';

const mock_addAction = Spy.mockModule('../../utils/store', 'addAction');

const makeLock = (id: string): Lock => ({
    message: id,
    lockId: id,
});

describe('EnvironmentChip', () => {
    const env: Environment = {
        name: 'Test Me',
        distanceToUpstream: 0,
        priority: Priority.PROD,
    };
    const allEnvLocks: GetAllEnvTeamLocksResponse = {
        allEnvLocks: {},
        allTeamLocks: {},
    };
    const envGroup: EnvironmentGroup = {
        distanceToUpstream: 0,
        environments: [env],
        environmentGroupName: 'Test Me Group',
        priority: Priority.PROD,
    };
    const getNode = (overloads?: Partial<EnvironmentChipProps>) => (
        <EnvironmentChip app="app2" className={'chip--test'} env={env} envGroup={envGroup} {...overloads} />
    );
    const getWrapper = (overloads?: Partial<EnvironmentChipProps>) => render(getNode(overloads));
    it('renders a chip', () => {
        // given
        UpdateOverview.set({
            environmentGroups: [
                {
                    environments: [env],
                    environmentGroupName: 'dontcare',
                    distanceToUpstream: 0,
                    priority: Priority.UNRECOGNIZED,
                },
            ],
        });
        updateAllEnvLocks.set(allEnvLocks);
        // then
        const { container } = getWrapper();
        expect(container.firstChild).toMatchInlineSnapshot(`
            <div
              class="mdc-evolution-chip chip--test environment-priority-prod"
              role="row"
            >
              <span
                class="mdc-evolution-chip__cell mdc-evolution-chip__cell--primary mdc-evolution-chip__action--primary"
                role="gridcell"
              >
                <span
                  class="mdc-evolution-chip__text-name"
                >
                  <span
                    class="env-card-header-name"
                  >
                    Test Me
                  </span>
                </span>
                 
                <span
                  class="mdc-evolution-chip__text-numbers"
                />
                <div
                  class="chip--test env-locks"
                />
              </span>
            </div>
        `);
    });
    it('renders a short form tag chip', () => {
        const allEnvLocks: GetAllEnvTeamLocksResponse = {
            allTeamLocks: {},
            allEnvLocks: {
                [env.name]: {
                    locks: [makeLock('lock1'), makeLock('lock2')],
                },
            },
        };
        updateAllEnvLocks.set(allEnvLocks);
        const wrapper = getWrapper({
            smallEnvChip: true,
            env: {
                ...env,
            },
        });
        const { container } = wrapper;
        expect(container.querySelector('.mdc-evolution-chip__text-name')?.textContent).toBe(env.name[0].toUpperCase());
        // only show one lock icon in the small env tag
        expect(container.querySelectorAll('.env-card-env-lock-icon').length).toBe(1);
    });
    it('renders env locks in big env chip', () => {
        const allEnvLocks: GetAllEnvTeamLocksResponse = {
            allTeamLocks: {},
            allEnvLocks: {
                [env.name]: {
                    locks: [makeLock('test-lock1'), makeLock('test-lock2')],
                },
            },
        };
        updateAllEnvLocks.set(allEnvLocks);
        UpdateOverview.set({
            environmentGroups: [
                {
                    environments: [
                        {
                            ...env,
                        },
                    ],
                    priority: Priority.UNRECOGNIZED,
                    environmentGroupName: 'dontcare',
                    distanceToUpstream: 0,
                },
            ],
        });

        const wrapper = getWrapper({
            // big chip shows all locks
            smallEnvChip: false,
            env: {
                ...env,
            },
        });

        const { container } = wrapper;
        expect(container.querySelectorAll('.env-card-env-lock-icon').length).toBe(2);
        const lock1 = container.querySelectorAll('.button-lock')[0];
        fireEvent.click(lock1);
        mock_addAction.addAction.wasCalled();
        expect(mock_addAction.addAction.getCallArguments()[0]).toHaveProperty(
            'action.deleteEnvironmentLock.lockId',
            'test-lock1'
        );
    });
});

const envGroupPairFromPrios = (
    envPrio: Priority,
    envGroupPrio: Priority
): { env: Environment; envGroup: EnvironmentGroup } => {
    const env: Environment = {
        distanceToUpstream: -1, // shouldn't matter, if this value is used an error will be thrown
        name: 'Test me',
        priority: envPrio,
    };
    const envGroup: EnvironmentGroup = {
        distanceToUpstream: -1, // shouldn't matter, if this value is used an error will be thrown
        environmentGroupName: 'Test me group',
        environments: [env],
        priority: envGroupPrio,
    };

    return { env, envGroup };
};

type TestDataEnvs = {
    envGroupPair: {
        env: Environment;
        envGroup: EnvironmentGroup;
    };
    expectedClass: string;
};

const envChipData: Array<TestDataEnvs> = [
    {
        envGroupPair: envGroupPairFromPrios(Priority.PROD, Priority.PROD),
        expectedClass: 'prod',
    },
    {
        envGroupPair: envGroupPairFromPrios(Priority.PRE_PROD, Priority.PRE_PROD),
        expectedClass: 'pre_prod',
    },
    {
        envGroupPair: envGroupPairFromPrios(Priority.UPSTREAM, Priority.UPSTREAM),
        expectedClass: 'upstream',
    },
    {
        envGroupPair: envGroupPairFromPrios(Priority.OTHER, Priority.OTHER),
        expectedClass: 'other',
    },
    {
        // important case: env and group have different priorities, the priority of the group should take precedence
        envGroupPair: envGroupPairFromPrios(Priority.UPSTREAM, Priority.PROD),
        expectedClass: 'prod',
    },
    {
        // important case: env and group have different priorities, the priority of the group should take precedence
        envGroupPair: envGroupPairFromPrios(Priority.PRE_PROD, Priority.CANARY),
        expectedClass: 'canary',
    },
];

describe.each(envChipData)(`EnvironmentChip with envPrio Classname`, (testcase) => {
    it(`with envPrio=${testcase.envGroupPair.env.priority} and groupPrio=${testcase.envGroupPair.envGroup.priority}`, () => {
        const env = testcase.envGroupPair.env;
        const group = testcase.envGroupPair.envGroup;
        const getNode = () => <EnvironmentChip app="app1" className={'chip--hello'} env={env} envGroup={group} />;
        const getWrapper = () => render(getNode());
        const { container } = getWrapper();
        expect(container.firstChild).toHaveClass(
            'mdc-evolution-chip chip--hello environment-priority-' + testcase.expectedClass
        );
    });
});

const envGroupFromPrio = (prio: Priority, numEnvsInGroup: number, envs: Environment[]): EnvironmentGroupExtended => ({
    numberOfEnvsInGroup: numEnvsInGroup,
    environmentGroupName: 'i am the group',
    environments: envs,
    distanceToUpstream: 0,
    priority: Priority.UNRECOGNIZED,
});

type TestDataGroups = {
    envGroup: EnvironmentGroupExtended;
    expectedClass: string;
    expectedNumbers: string;
    expectedDisplayName: string;
};

const envFromPrio = (prio: Priority): Environment => ({
    name: 'Test Me',
    distanceToUpstream: 0,
    priority: prio,
});

const envGroupChipData: Array<TestDataGroups> = [
    {
        envGroup: envGroupFromPrio(Priority.PROD, 1, [envFromPrio(Priority.PROD)]),
        expectedClass: 'prod',
        expectedNumbers: '(1)',
        expectedDisplayName: 'Test Me',
    },
    {
        envGroup: envGroupFromPrio(Priority.PROD, 3, [envFromPrio(Priority.UPSTREAM), envFromPrio(Priority.PROD)]),
        expectedClass: 'upstream',
        expectedNumbers: '(2/3)',
        expectedDisplayName: 'i am the group',
    },
];

describe.each(envGroupChipData)(`EnvironmentGroupChip with different envs`, (testcase) => {
    it(`with envPrio=${testcase.expectedClass}`, () => {
        const getNode = () => (
            <EnvironmentGroupChip app="app1" className={'chip--hello'} envGroup={testcase.envGroup} />
        );
        const getWrapper = () => render(getNode());
        const { container } = getWrapper();
        expect(container.querySelector('.mdc-evolution-chip__text-name')?.textContent).toContain(
            testcase.expectedDisplayName
        );
        expect(container.querySelector('.mdc-evolution-chip__text-numbers')?.textContent).toContain(
            testcase.expectedNumbers
        );
    });
});
