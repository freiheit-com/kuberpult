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
import { act, render } from '@testing-library/react';
import { DotsMenu, DotsMenuProps } from './DotsMenu';
import { elementQuerySelectorSafe } from '../../../setupTests';

describe('DotsMenu Rendering', () => {
    const getNode = (overrides: DotsMenuProps) => <DotsMenu {...overrides} />;
    const getWrapper = (overrides: DotsMenuProps) => render(getNode(overrides));

    const mySpy = jest.fn();

    type TestData = {
        name: string;
        input: DotsMenuProps;
        expectedNumItems: number;
    };

    const data: TestData[] = [
        {
            name: 'renders empty list',
            input: { buttons: [] },
            expectedNumItems: 0,
        },
        {
            name: 'renders one button',
            input: {
                buttons: [
                    {
                        label: 'test label',
                        onClick: mySpy,
                    },
                ],
            },
            expectedNumItems: 1,
        },
        {
            name: 'renders three button',
            input: {
                buttons: [
                    {
                        label: 'test label A',
                        onClick: mySpy,
                    },
                    {
                        label: 'test label B',
                        onClick: mySpy,
                    },
                    {
                        label: 'test label C',
                        onClick: mySpy,
                    },
                ],
            },
            expectedNumItems: 3,
        },
    ];

    describe.each(data)('DotsMenu Test', (testcase) => {
        it(testcase.name, () => {
            mySpy.mockReset();
            expect(mySpy).toHaveBeenCalledTimes(0);

            const { container } = getWrapper(testcase.input);

            expect(document.querySelectorAll('.dots-menu-hidden .mdc-button--unelevated').length).toEqual(1);
            const result = elementQuerySelectorSafe(container, '.dots-menu-hidden .mdc-button--unelevated');
            act(() => {
                result.click();
            });
            expect(document.querySelectorAll('.dots-menu-hidden .mdc-button--unelevated').length).toEqual(0);
            expect(document.querySelectorAll('li.item').length).toEqual(testcase.expectedNumItems);

            if (testcase.expectedNumItems > 0) {
                expect(mySpy).toHaveBeenCalledTimes(0);
                const result = elementQuerySelectorSafe(container, 'li .mdc-button--unelevated');
                act(() => {
                    result.click();
                });
                expect(mySpy).toHaveBeenCalledTimes(1);
            }
        });
    });
});

const addSibling = (wrappee: JSX.Element) => (
    <>
        {wrappee}
        <div id="sibling-outside"></div>
    </>
);

describe('DotsMenu Close', () => {
    const getNode = (overrides: DotsMenuProps) => <DotsMenu {...overrides} />;
    const getWrapper = (overrides: DotsMenuProps) => render(addSibling(getNode(overrides)));

    type propMod = (props: Partial<DotsMenuProps>) => void;
    const fixtureProps = (...mods: propMod[]) => {
        const props = {
            buttons: [
                {
                    label: 'test label A',
                    onClick: () => {},
                },
                {
                    label: 'test label B',
                    onClick: () => {},
                },
                {
                    label: 'test label C',
                    onClick: () => {},
                },
            ],
        };
        mods.forEach((m) => m(props));
        return props;
    };

    type TestData = {
        name: string;
        props: DotsMenuProps;
        sendKey: string;
        clickViaEvent: string[];
        expectClosed: boolean;
    };
    const fixtureTestData = () => ({
        props: fixtureProps(),
        sendKey: '',
        clickViaEvent: [],
        expectClosed: false,
    });

    const data: TestData[] = [
        {
            name: 'visible if not clicking',
            ...fixtureTestData(),
            expectClosed: false,
        },
        {
            name: 'close if escape is pressed',
            ...fixtureTestData(),
            sendKey: 'Escape',
            expectClosed: true,
        },
        {
            name: 'visible if any other button is pressed',
            ...fixtureTestData(),
            sendKey: 'a',
            expectClosed: false,
        },
        {
            name: 'close if sibling is clicked',
            ...fixtureTestData(),
            clickViaEvent: ['#sibling-outside'],
            expectClosed: true,
        },
        {
            name: 'visible if elements inside is clicked',
            ...fixtureTestData(),
            clickViaEvent: ['.item', '.dots-menu-open'],
            expectClosed: false,
        },
    ];

    describe.each(data)('DotsMenu Closing', (testcase) => {
        it(testcase.name, () => {
            const { container } = getWrapper(testcase.props);

            // open menu
            expect(document.querySelectorAll('.dots-menu-hidden .mdc-button--unelevated').length).toEqual(1);
            const result = elementQuerySelectorSafe(container, '.dots-menu-hidden .mdc-button--unelevated');
            act(() => {
                result.click();
            });
            expect(document.querySelectorAll('.dots-menu-hidden .mdc-button--unelevated').length).toEqual(0);

            testcase.clickViaEvent.forEach((selector) => {
                act(() => {
                    const elem = elementQuerySelectorSafe(container, selector);
                    ['pointerdown', 'mousedown', 'click', 'mouseup', 'pointerup'].forEach((mouseEventType) => {
                        elem.dispatchEvent(
                            new MouseEvent(mouseEventType, {
                                view: window,
                                bubbles: true,
                                buttons: 1,
                            })
                        );
                    });
                });
            });

            if (testcase.sendKey !== '') {
                ['keydown', 'keypress', 'keyup'].forEach((keyEventType) =>
                    act(() =>
                        document.dispatchEvent(
                            new KeyboardEvent(keyEventType, { key: testcase.sendKey, bubbles: true })
                        )
                    )
                );
            }

            if (testcase.expectClosed) {
                expect(document.querySelectorAll('.dots-menu-hidden .mdc-button--unelevated').length).toEqual(1);
            } else {
                expect(document.querySelectorAll('.dots-menu-open').length).toEqual(1);
            }
        });
    });
});
