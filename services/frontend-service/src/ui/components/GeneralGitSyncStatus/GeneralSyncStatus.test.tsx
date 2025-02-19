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
import React from 'react';
import { render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { GeneralGitSyncStatus } from './GeneralSyncStatus';
import { UpdateGitSyncStatus } from '../../utils/store';
import { GetGitSyncStatusResponse, GitSyncStatus } from '../../../api/api';

describe('GeneralSyncStatus', () => {
    const getNode = () => (
        <MemoryRouter>
            <GeneralGitSyncStatus enabled={true} />
        </MemoryRouter>
    );
    const getWrapper = () => render(getNode());

    interface dataEnvT {
        name: string;
        gitSyncStatus: GetGitSyncStatusResponse;
        expectedClassName: string;
    }
    const sampleGitSyncData: dataEnvT[] = [
        {
            name: 'No data means sync is shown',
            gitSyncStatus: {
                appStatuses: {},
            },
            expectedClassName: 'general-status__synced',
        },
        {
            name: 'Solo synced reveals synced',
            gitSyncStatus: {
                appStatuses: {
                    app1: {
                        envStatus: {
                            development: GitSyncStatus.GIT_SYNC_STATUS_SYNCED,
                        },
                    },
                },
            },
            expectedClassName: 'general-status__synced',
        },
        {
            name: 'Solo unsynced reveals unsynced',
            gitSyncStatus: {
                appStatuses: {
                    app1: {
                        envStatus: {
                            development: GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED,
                        },
                    },
                },
            },
            expectedClassName: 'general-status__unsynced',
        },
        {
            name: 'Solo error reveals error',
            gitSyncStatus: {
                appStatuses: {
                    app1: {
                        envStatus: {
                            development: GitSyncStatus.GIT_SYNC_STATUS_ERROR,
                        },
                    },
                },
            },
            expectedClassName: 'general-status__error',
        },
        {
            name: 'Shows highest priority',
            gitSyncStatus: {
                appStatuses: {
                    app1: {
                        envStatus: {
                            development: GitSyncStatus.GIT_SYNC_STATUS_SYNCED,
                        },
                    },
                    app2: {
                        envStatus: {
                            development: GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED,
                        },
                    },
                },
            },
            expectedClassName: 'general-status__unsynced',
        },
        {
            name: 'Shows highest priority between all',
            gitSyncStatus: {
                appStatuses: {
                    app1: {
                        envStatus: {
                            development: GitSyncStatus.GIT_SYNC_STATUS_SYNCED,
                            staging: GitSyncStatus.GIT_SYNC_STATUS_ERROR,
                        },
                    },
                    app2: {
                        envStatus: {
                            development: GitSyncStatus.GIT_SYNC_STATUS_UNSYNCED,
                            staging: GitSyncStatus.GIT_SYNC_STATUS_UNKNOWN,
                        },
                    },
                },
            },
            expectedClassName: 'general-status__error',
        },
    ];
    describe.each(sampleGitSyncData)(`git sync data`, (tc) => {
        it(tc.name, () => {
            UpdateGitSyncStatus(tc.gitSyncStatus);
            const { container } = getWrapper();
            expect(container.getElementsByClassName(tc.expectedClassName)).toHaveLength(1);
        });
    });
});
