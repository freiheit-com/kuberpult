/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
import {EnvironmentChip, EnvironmentGroupChip} from './EnvironmentGroupChip';
import {render} from '@testing-library/react';
import {Environment, EnvironmentGroup, Priority} from '../../../api/api';
import {EnvironmentGroupExtended} from "../../utils/store";

describe('EnvironmentChip', () => {
    const env: Environment = {
        name: 'Test Me',
        distanceToUpstream: 0,
        priority: Priority.PROD,
        locks: {},
        applications: {},
    };
    const getNode = () => <EnvironmentChip className={'chip--test'} env={env} withEnvLocks={false} />;
    const getWrapper = () => render(getNode());
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
                  class="mdc-evolution-chip__text-label"
                >
                  Test Me
                   
                </span>
              </span>
            </div>
        `);
    });
});

const envFromPrio = (prio: Priority): Environment => {
    return {
        name: 'Test Me',
        distanceToUpstream: 0,
        priority: prio,
        locks: {},
        applications: {},
    };
};

type TestDataEnvs = {
    env: Environment,
    expectedClass: string,
}

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

const envGroupFromPrio = (prio: Priority, envs: Environment[]): EnvironmentGroupExtended => {
    return {
        numberOfEnvsInGroup: envs.length,
        environmentGroupName: "",
        environments: envs,
        distanceToUpstream: 0
    };
};


type TestDataGroups = {
    envGroup: EnvironmentGroupExtended,
    expectedClass: string,
}

const envGroupChipData: Array<TestDataGroups> = [
    {
        envGroup: envGroupFromPrio(Priority.PROD, [envFromPrio(Priority.PROD)]),
        expectedClass: 'prod',
    },
    // {
    //     env: envFromPrio(Priority.PRE_PROD),
    //     expectedClass: 'pre_prod',
    // },
    // {
    //     env: envFromPrio(Priority.UPSTREAM),
    //     expectedClass: 'upstream',
    // },
    // {
    //     env: envFromPrio(Priority.OTHER),
    //     expectedClass: 'other',
    // },
];

describe.each(envChipData)(`EnvironmentChip with envPrio Classname`, (testcase) => {
    it(`with envPrio=${testcase.env.priority}`, () => {
        const getNode = () => (
            <EnvironmentChip className={'chip--hello'} env={testcase.env}  withEnvLocks={false} />
        );
        const getWrapper = () => render(getNode());
        const { container } = getWrapper();
        expect(container.firstChild).toHaveClass(
            'mdc-evolution-chip chip--hello chip--hello-' + testcase.expectedClass
        );
    });
});


describe.each(envGroupChipData)(`EnvironmentGroupChip with envPrio Classname`, (testcase) => {
    it(`with envPrio=${testcase.expectedClass}`, () => {
        const getNode = () => (
            <EnvironmentGroupChip className={'chip--hello'} envGroup={testcase.envGroup} />
        );
        const getWrapper = () => render(getNode());
        const { container } = getWrapper();
        expect(container.firstChild).toHaveClass(
            'mdc-evolution-chip chip--hello chip--hello-' + testcase.expectedClass
        );
    });
});
