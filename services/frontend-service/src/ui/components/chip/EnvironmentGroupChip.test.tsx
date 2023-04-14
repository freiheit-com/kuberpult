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

Copyright 2023 freiheit.com*/
import { EnvironmentChip, EnvironmentChipProps, EnvironmentGroupChip } from './EnvironmentGroupChip';
import { fireEvent, render } from '@testing-library/react';
import { Environment, Lock, Priority } from '../../../api/api';
import { EnvironmentGroupExtended } from '../../utils/store';
import { Spy } from 'spy4js';

const mock_addAction = Spy.mockModule('../../utils/store', 'addAction');

describe('EnvironmentChip', () => {
    const env: Environment = {
        name: 'Test Me',
        distanceToUpstream: 0,
        priority: Priority.PROD,
        locks: {},
        applications: {},
    };
    const getNode = (overloads?: EnvironmentChipProps) => (
        <EnvironmentChip className={'chip--test'} env={env} {...overloads} />
    );
    const getWrapper = (overloads?: EnvironmentChipProps) => render(getNode(overloads));
    it('renders a chip', () => {
        const { container } = getWrapper();
        expect(container.firstChild).toMatchInlineSnapshot(`
            <div
              class="mdc-evolution-chip chip--test chip--test-prod"
              role="row"
            >
              <span
                class="mdc-evolution-chip__cell mdc-evolution-chip__cell--primary mdc-evolution-chip__action--primary"
                role="gridcell"
              >
                <span
                  class="mdc-evolution-chip__text-name"
                >
                  Test Me
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
        // eslint-disable-next-line no-type-assertion/no-type-assertion
        const { container } = getWrapper({
            smallEnvChip: true,
            // eslint-disable-next-line no-type-assertion/no-type-assertion
            env: { ...env, locks: [{ lockId: 'lock1' }, { lockId: 'lock2' }] as Lock[] },
        } as any);
        expect(container.querySelector('.mdc-evolution-chip__text-name')?.textContent).toBe(env.name[0].toUpperCase());
        // only show one lock icon in the small env tag
        expect(container.querySelectorAll('.env-card-env-lock-icon').length).toBe(1);
    });
    it('renders env locks in big env chip', () => {
        // eslint-disable-next-line no-type-assertion/no-type-assertion
        const { container } = getWrapper({
            // eslint-disable-next-line no-type-assertion/no-type-assertion
            env: { ...env, locks: [{ lockId: 'test-lock1' }, { lockId: 'test-lock2' }] as Lock[] },
        } as any);
        // big chip shows all locks
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

const envFromPrio = (prio: Priority): Environment => ({
    name: 'Test Me',
    distanceToUpstream: 0,
    priority: prio,
    locks: {},
    applications: {},
});

type TestDataEnvs = {
    env: Environment;
    expectedClass: string;
};

const envChipData: Array<TestDataEnvs> = [
    {
        env: envFromPrio(Priority.PROD),
        expectedClass: 'prod',
    },
    {
        env: envFromPrio(Priority.PRE_PROD),
        expectedClass: 'pre_prod',
    },
    {
        env: envFromPrio(Priority.UPSTREAM),
        expectedClass: 'upstream',
    },
    {
        env: envFromPrio(Priority.OTHER),
        expectedClass: 'other',
    },
];

describe.each(envChipData)(`EnvironmentChip with envPrio Classname`, (testcase) => {
    it(`with envPrio=${testcase.env.priority}`, () => {
        const getNode = () => <EnvironmentChip className={'chip--hello'} env={testcase.env} />;
        const getWrapper = () => render(getNode());
        const { container } = getWrapper();
        expect(container.firstChild).toHaveClass(
            'mdc-evolution-chip chip--hello chip--hello-' + testcase.expectedClass
        );
    });
});

const envGroupFromPrio = (prio: Priority, numEnvsInGroup: number, envs: Environment[]): EnvironmentGroupExtended => ({
    numberOfEnvsInGroup: numEnvsInGroup,
    environmentGroupName: 'i am the group',
    environments: envs,
    distanceToUpstream: 0,
});

type TestDataGroups = {
    envGroup: EnvironmentGroupExtended;
    expectedClass: string;
    expectedNumbers: string;
    expectedDisplayName: string;
};

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
        const getNode = () => <EnvironmentGroupChip className={'chip--hello'} envGroup={testcase.envGroup} />;
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
