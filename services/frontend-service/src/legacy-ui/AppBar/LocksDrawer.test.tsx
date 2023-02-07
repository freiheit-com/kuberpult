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
import React from 'react';
import { fireEvent, getAllByText, getByText, render } from '@testing-library/react';

import { LocksDrawer } from './LocksDrawer';
import { GetOverviewResponse } from '../../api/api';

describe('All locks drawer', () => {
    const dump: GetOverviewResponse = {
        applications: {},
        environments: {
            development: {
                applications: {},
                config: {},
                name: 'development',
                locks: {
                    'ui-3vycs8': {
                        lockId: 'ui-3vycs8',
                        createdBy: {
                            email: 'test@test.com',
                            name: 'tester',
                        },
                        createdAt: new Date('Thu Dec 16 2021 15:49:13 GMT+0100 (Central European Standard Time)'),
                        message: 'test',
                    },
                    'ui-cw3wdp': {
                        lockId: 'ui-cw3wdp',
                        createdBy: {
                            email: 'test2@test.com',
                            name: 'tester2',
                        },
                        createdAt: new Date('Thu Dec 16 2021 11:15:34 GMT+0100 (Central European Standard Time)'),
                        message: 'test2',
                    },
                },
            },
        },
    };
    const getNode = (overrides?: { data: GetOverviewResponse }): JSX.Element | any => {
        // given
        const defaultProps: any = {
            children: null,
            data: dump,
        };
        return <LocksDrawer {...defaultProps} {...overrides} />;
    };
    const getWrapper = (overrides?: { data: GetOverviewResponse }) => render(getNode(overrides));

    it('renders the LocksDrawer component', () => {
        // when
        const { container } = getWrapper();
        // then
        expect(getByText(container, /locks/i)).toBeTruthy();
    });

    it('LocksDrawer badge show warning at least one lock older than 2 days ', () => {
        // when
        const { container } = getWrapper();
        // then
        expect(container.querySelector('.MuiSvgIcon-colorError')).toBeTruthy();
    });

    const noLocks: GetOverviewResponse = {
        applications: {},
        environments: {
            development: {
                applications: {},
                config: {},
                name: 'development',
                locks: {},
            },
        },
    };
    it('LocksDrawer should show message if there are no locks ', () => {
        // when
        const { container } = getWrapper({ data: noLocks });

        //fire event
        fireEvent.click(container.querySelector('.MuiButton-root')!);

        const d = document.querySelector('.MuiDrawer-root') as HTMLElement;
        // then
        expect(getAllByText(d, /There are no locks here yet!/i).length).toBe(2);
    });

    const data: GetOverviewResponse = {
        applications: {},
        environments: {
            development: {
                applications: {},
                config: {},
                name: 'development',
                locks: {
                    'ui-3vycs8': {
                        lockId: 'ui-3vycs8',
                        createdBy: {
                            email: 'test@test.com',
                            name: 'tester',
                        },
                        createdAt: new Date(Date.now()),
                        message: 'test',
                    },
                    'ui-cw3wdp': {
                        lockId: 'ui-cw3wdp',
                        createdBy: {
                            email: 'test2@test.com',
                            name: 'tester2',
                        },
                        createdAt: new Date(Date.now()),
                        message: 'test2',
                    },
                },
            },
        },
    };

    it("LocksDrawer badge doesn't show warning when all locks newer than 2 days ", () => {
        // when
        const { container } = getWrapper({ data });
        // then
        expect(container.querySelector('.MuiSvgIcon-colorError')).not.toBeTruthy();
    });
});

describe('Calc Lock Age', () => {
    interface dataT {
        act: {
            action: {
                age: number;
            };
        };
        fin?: () => void;
        expect: {
            text: string;
        };
    }

    const data: dataT[] = [
        {
            act: {
                action: {
                    age: 0,
                },
            },
            expect: {
                text: '< 1 day ago',
            },
        },
        {
            act: {
                action: {
                    age: 1,
                },
            },
            expect: {
                text: '1 day ago',
            },
        },
        {
            act: {
                action: {
                    age: 2,
                },
            },
            expect: {
                text: '2 days ago',
            },
        },
        {
            act: {
                action: {
                    age: 14,
                },
            },
            expect: {
                text: '14 days ago',
            },
        },
    ];

    describe.each(data)('Age lock calculator', (testcase: dataT) => {
        it(`Lock age ${testcase.act.action.age}`, () => {
            const calcLockAge = (age: number): string => {
                if (age <= 1) return `${age === 0 ? '< 1' : '1'} day ago`;
                return `${age} days ago`;
            };

            expect(calcLockAge(testcase.act.action.age)).toStrictEqual(testcase.expect.text);
        });
    });
});
