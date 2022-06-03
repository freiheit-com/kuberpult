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
import { CartAction, getActionDetails, transformToBatch } from './ActionDetails';
import { BatchAction, LockBehavior } from '../api/api';

const mockMath = Object.create(global.Math);
mockMath.random = () => 0.5136516832518615;
global.Math = mockMath;

const sampleLockMessage = 'foo';
const sampleLockId = 'ui-2yja'; // from mocked Math.Random

const sampleDeployAction: CartAction = {
    deploy: {
        application: 'dummy application',
        version: 22,
        environment: 'dummy environment',
    },
};

const transformedDeployAction: BatchAction = {
    action: {
        $case: 'deploy',
        deploy: {
            application: 'dummy application',
            version: 22,
            environment: 'dummy environment',
            lockBehavior: LockBehavior.Ignore,
            ignoreAllLocks: false,
        },
    },
};

const sampleUndeployAction: CartAction = {
    undeploy: {
        application: 'dummy application',
    },
};

const transformedUndeployAction: BatchAction = {
    action: {
        $case: 'undeploy',
        undeploy: {
            application: 'dummy application',
        },
    },
};

const sampleCreateEnvLock: CartAction = {
    createEnvironmentLock: {
        environment: 'dummy environment',
    },
};

const transformedCreateEnvLock: BatchAction = {
    action: {
        $case: 'createEnvironmentLock',
        createEnvironmentLock: {
            environment: 'dummy environment',
            lockId: sampleLockId,
            message: sampleLockMessage,
        },
    },
};

const sampleDeleteEnvLock: CartAction = {
    deleteEnvironmentLock: {
        environment: 'dummy environment',
        lockId: sampleLockId,
    },
};

const transformedDeleteEnvLock: BatchAction = {
    action: {
        $case: 'deleteEnvironmentLock',
        deleteEnvironmentLock: {
            environment: 'dummy environment',
            lockId: sampleLockId,
        },
    },
};

const sampleCreateAppLock: CartAction = {
    createApplicationLock: {
        application: 'dummy application',
        environment: 'dummy environment',
    },
};

const transformedCreateAppLock: BatchAction = {
    action: {
        $case: 'createEnvironmentApplicationLock',
        createEnvironmentApplicationLock: {
            application: 'dummy application',
            environment: 'dummy environment',
            lockId: sampleLockId,
            message: sampleLockMessage,
        },
    },
};

const sampleDeleteAppLock: CartAction = {
    deleteApplicationLock: {
        application: 'dummy application',
        environment: 'dummy environment',
        lockId: sampleLockId,
    },
};

const transformedDeleteAppLock: BatchAction = {
    action: {
        $case: 'deleteEnvironmentApplicationLock',
        deleteEnvironmentApplicationLock: {
            application: 'dummy application',
            environment: 'dummy environment',
            lockId: sampleLockId,
        },
    },
};

describe('Action Details Logic', () => {
    interface dataT {
        type: string;
        act: CartAction;
        transformed: BatchAction;
    }

    const data: dataT[] = [
        {
            type: 'Deploy',
            act: sampleDeployAction,
            transformed: transformedDeployAction,
        },
        {
            type: 'Undeploy',
            act: sampleUndeployAction,
            transformed: transformedUndeployAction,
        },
        {
            type: 'Create Env Lock',
            act: sampleCreateEnvLock,
            transformed: transformedCreateEnvLock,
        },
        {
            type: 'Delete Env Lock',
            act: sampleDeleteEnvLock,
            transformed: transformedDeleteEnvLock,
        },
        {
            type: 'Create App Lock',
            act: sampleCreateAppLock,
            transformed: transformedCreateAppLock,
        },
        {
            type: 'Delete App Lock',
            act: sampleDeleteAppLock,
            transformed: transformedDeleteAppLock,
        },
    ];

    describe.each(data)(`Cart Action Types`, (testcase: dataT) => {
        it(`${testcase.type} is transformed correctly`, () => {
            expect(getActionDetails(testcase.act).name).toBe(testcase.type);
            expect(transformToBatch(testcase.act, sampleLockMessage)).toStrictEqual(testcase.transformed);
        });
    });
});
