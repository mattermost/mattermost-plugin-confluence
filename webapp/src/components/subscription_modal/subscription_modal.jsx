import React from 'react';
import {
    Modal,
    Button,
} from 'react-bootstrap';

import PropTypes from 'prop-types';

import Constants from '../../constants';
import ConfluenceField from '../confluence_field';
import Validator from '../validator';

const initialState = {
    alias: '',
    baseURL: '',
    spaceKey: '',
    pageID: '',
    subscriptionType: Constants.SUBSCRIPTION_TYPE[0],
    events: Constants.CONFLUENCE_EVENTS,
    supportedEvents: Constants.CONFLUENCE_EVENTS,
    error: '',
    saving: false,
};

export default class SubscriptionModal extends React.PureComponent {
    static propTypes = {
        visibility: PropTypes.bool,
        subscription: PropTypes.object,
        close: PropTypes.func.isRequired,
        theme: PropTypes.object.isRequired,
        saveChannelSubscription: PropTypes.func.isRequired,
        currentChannelID: PropTypes.string.isRequired,
        editChannelSubscription: PropTypes.func.isRequired,
        getPluginConfig: PropTypes.func.isRequired,
    };

    static defaultProps = {
        visibility: false,
        subscription: {},
    };

    constructor(props) {
        super(props);
        this.state = initialState;
        this.validator = new Validator();
    }

    async componentDidMount() {
        await this.loadPluginConfig();
    }

    componentDidUpdate(prevProps) {
        if (this.props.subscription !== prevProps.subscription) {
            this.setData();
        }
    }

    loadPluginConfig = async () => {
        const result = await this.props.getPluginConfig();
        this.setState({
            supportedEvents: result?.data?.supportedEvents || Constants.CONFLUENCE_EVENTS,
            events: result?.data?.supportedEvents || Constants.CONFLUENCE_EVENTS,
        }, () => {
            if (this.props.subscription?.alias) {
                this.setData();
            }
        });
    }

    setData = () => {
        const {
            alias, baseURL, spaceKey, events, pageID,
        } = this.props.subscription;
        if (alias) {
            const availableEvents = this.state.supportedEvents.filter((option) => events.includes(option.value));
            this.setState({
                alias,
                baseURL,
                spaceKey,
                pageID,
                events: availableEvents,
                subscriptionType: pageID ? Constants.SUBSCRIPTION_TYPE[1] : Constants.SUBSCRIPTION_TYPE[0],
            });
        }
    };

    handleClose = (e) => {
        if (e && e.preventDefault) {
            e.preventDefault();
        }

        // Preserve supportedEvents from the config, only reset form fields
        const {supportedEvents} = this.state;
        this.setState({
            ...initialState,
            supportedEvents,
            events: supportedEvents,
        }, this.props.close);
    };

    handleAlias = (e) => {
        this.setState({
            alias: e.target.value,
        });
    };

    handleBaseURLChange = (e) => {
        this.setState({
            baseURL: e.target.value.toLowerCase(),
        });
    };

    handleSpaceKey = (e) => {
        this.setState({
            spaceKey: e.target.value,
        });
    };

    handlePageID = (e) => {
        this.setState({
            pageID: e.target.value,
        });
    };

    handleEvents = (events) => {
        this.setState({
            events,
        });
    };

    handleSubscriptionType = (subscriptionType) => {
        if (subscriptionType === this.state.subscriptionType) {
            return;
        }
        this.setState({
            subscriptionType,
            pageID: '',
            spaceKey: '',
        });
    };

    handleSubmit = async () => {
        if (!this.validator.validate()) {
            return;
        }
        const {
            alias, baseURL, spaceKey, events, pageID, subscriptionType,
        } = this.state;
        const {
            currentChannelID, subscription, saveChannelSubscription, editChannelSubscription,
        } = this.props;
        const channelSubscription = {
            subscriptionType: subscriptionType.value,
            alias: alias.trim(),
            baseURL: baseURL.trim().toLowerCase(),
            spaceKey: spaceKey ? spaceKey.trim() : '',
            pageID: pageID ? pageID.trim() : '',
            channelID: currentChannelID,
            events: events ? events.map((event) => event.value) : [],
        };
        this.setState({
            saving: true,
            error: '',
        });

        let response;
        if (subscription && subscription.alias) {
            channelSubscription.oldAlias = subscription.alias;
            response = await editChannelSubscription(channelSubscription);
        } else {
            response = await saveChannelSubscription(channelSubscription);
        }
        if (response.error) {
            this.setState({
                error: response.error.response.text,
                saving: false,
            });
            return;
        }
        this.handleClose();
    };

    render() {
        const {visibility, subscription} = this.props;
        const editSubscription = Boolean(subscription && subscription.alias);
        const isModalVisible = Boolean(visibility || editSubscription);
        const {error, saving, subscriptionType, supportedEvents, events} = this.state;
        let typeField = (
            <ConfluenceField
                formGroupStyle={getStyle.typeValue}
                formControlStyle={getStyle.typeFormControl}
                label={'Space Key'}
                type={'text'}
                fieldType={'input'}
                required={true}
                placeholder={'Enter the Confluence Space Key.'}
                value={this.state.spaceKey}
                addValidation={this.validator.addValidation}
                removeValidation={this.validator.removeValidation}
                onChange={this.handleSpaceKey}
            />
        );
        if (subscriptionType === Constants.SUBSCRIPTION_TYPE[1]) {
            typeField = (
                <ConfluenceField
                    formGroupStyle={getStyle.typeValue}
                    formControlStyle={getStyle.typeFormControl}
                    label={'Page ID'}
                    type={'text'}
                    fieldType={'input'}
                    required={true}
                    placeholder={'Enter the page id.'}
                    value={this.state.pageID}
                    addValidation={this.validator.addValidation}
                    removeValidation={this.validator.removeValidation}
                    onChange={this.handlePageID}
                />
            );
        }
        const innerFields = (
            <div style={getStyle.innerFields}>
                <ConfluenceField
                    formGroupStyle={getStyle.subscriptionType}
                    isSearchable={false}
                    isMulti={false}
                    label={'Subscribe To'}
                    name={'type'}
                    fieldType={'dropDown'}
                    required={true}
                    theme={this.props.theme}
                    options={Constants.SUBSCRIPTION_TYPE}
                    value={this.state.subscriptionType}
                    addValidation={this.validator.addValidation}
                    removeValidation={this.validator.removeValidation}
                    onChange={this.handleSubscriptionType}
                />
                {typeField}
            </div>
        );
        let createError = null;
        if (error) {
            createError = (
                <p className='alert alert-danger'>
                    <i
                        className='fa fa-warning'
                        title='Warning Icon'
                    />
                    <span> {error}</span>
                </p>
            );
        }

        return (
            <Modal
                show={isModalVisible}
                onHide={this.handleClose}
                backdrop={'static'}
            >
                <Modal.Header closeButton={true}>
                    <Modal.Title>
                        {'Edit Your Confluence Subscription'}
                    </Modal.Title>
                </Modal.Header>
                <Modal.Body>
                    <div>
                        <ConfluenceField
                            label={'Name'}
                            type={'text'}
                            fieldType={'input'}
                            required={true}
                            placeholder={'Enter a name for this subscription.'}
                            value={this.state.alias}
                            addValidation={this.validator.addValidation}
                            removeValidation={this.validator.removeValidation}
                            onChange={this.handleAlias}
                        />
                        <ConfluenceField
                            label={'Confluence Base URL'}
                            type={'text'}
                            fieldType={'input'}
                            required={true}
                            placeholder={'Enter the Confluence Base URL.'}
                            value={this.state.baseURL}
                            addValidation={this.validator.addValidation}
                            removeValidation={this.validator.removeValidation}
                            onChange={this.handleBaseURLChange}
                        />
                        {innerFields}
                        <ConfluenceField
                            isMulti={true}
                            label={'Events'}
                            name={'events'}
                            fieldType={'dropDown'}
                            required={true}
                            theme={this.props.theme}
                            options={supportedEvents}
                            value={events}
                            addValidation={this.validator.addValidation}
                            removeValidation={this.validator.removeValidation}
                            onChange={this.handleEvents}
                        />
                        {createError}
                    </div>
                </Modal.Body>
                <Modal.Footer>
                    <Button
                        type='button'
                        bsStyle='link'
                        onClick={this.handleClose}
                    >
                        {'Cancel'}
                    </Button>
                    <Button
                        type='submit'
                        bsStyle='primary'
                        onClick={this.handleSubmit}
                        disabled={saving}
                    >
                        {saving && <span className='fa fa-spinner fa-fw fa-pulse spinner'/>}
                        {'Save Subscription'}
                    </Button>
                </Modal.Footer>
            </Modal>
        );
    }
}

const getStyle = {
    innerFields: {
        display: 'flex',
    },
    subscriptionType: {
        flex: '1',
        marginRight: '20px',
    },
    typeValue: {
        flex: '1',
    },
    typeFormControl: {
        height: '38px',
    },
};
